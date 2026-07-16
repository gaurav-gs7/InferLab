package adapter

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gaurav-gs7/InferLab/internal/strictjson"
	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

var (
	namePattern     = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,126}[a-z0-9])?$`)
	revisionPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	digestPattern   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

const maxCapabilityMetrics = 4096

func ValidateCapabilities(capabilities Capabilities) error {
	if capabilities.Schema != ProtocolSchema || capabilities.SchemaVersion != CurrentVersion {
		return fmt.Errorf("%w: unsupported schema or version", ErrInvalidCapabilities)
	}
	if !namePattern.MatchString(capabilities.Adapter.Name) || !revisionPattern.MatchString(capabilities.Adapter.Revision) {
		return fmt.Errorf("%w: adapter identity is not immutable", ErrInvalidCapabilities)
	}
	if err := validateProducer(capabilities.Producer); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidCapabilities, err)
	}
	if len(capabilities.Classifications) == 0 || len(capabilities.Metrics) == 0 || len(capabilities.Metrics) > maxCapabilityMetrics {
		return fmt.Errorf("%w: classifications must not be empty and metrics must contain 1..%d entries", ErrInvalidCapabilities, maxCapabilityMetrics)
	}
	seenClasses := make(map[evidence.Classification]struct{}, len(capabilities.Classifications))
	for _, classification := range capabilities.Classifications {
		if classification != evidence.ClassObserved && classification != evidence.ClassPredicted {
			return fmt.Errorf("%w: adapters may emit only observed or predicted evidence", ErrInvalidCapabilities)
		}
		if _, exists := seenClasses[classification]; exists {
			return fmt.Errorf("%w: duplicate classification %q", ErrInvalidCapabilities, classification)
		}
		seenClasses[classification] = struct{}{}
	}
	seenSource := make(map[string]struct{}, len(capabilities.Metrics))
	seenTarget := make(map[string]struct{}, len(capabilities.Metrics))
	for i, metric := range capabilities.Metrics {
		if !namePattern.MatchString(metric.SourceName) || !namePattern.MatchString(metric.SourceDefinition) ||
			!namePattern.MatchString(metric.NormalizedName) || !namePattern.MatchString(metric.Semantics) {
			return fmt.Errorf("%w: metrics[%d] contains an invalid name", ErrInvalidCapabilities, i)
		}
		if !validIdentifier(metric.SourceUnit) || !validIdentifier(metric.NormalizedUnit) {
			return fmt.Errorf("%w: metrics[%d] requires explicit units", ErrInvalidCapabilities, i)
		}
		sourceKey := metric.SourceName + "\x00" + metric.SourceDefinition + "\x00" + metric.SourceUnit
		if _, exists := seenSource[sourceKey]; exists {
			return fmt.Errorf("%w: duplicate source metric %q", ErrInvalidCapabilities, metric.SourceName)
		}
		seenSource[sourceKey] = struct{}{}
		targetKey := metric.NormalizedName + "\x00" + metric.Semantics
		if _, exists := seenTarget[targetKey]; exists {
			return fmt.Errorf("%w: duplicate normalized metric %q", ErrInvalidCapabilities, metric.NormalizedName)
		}
		seenTarget[targetKey] = struct{}{}
	}
	return nil
}

func ValidateInput(input Input) error {
	if !namePattern.MatchString(input.Name) {
		return fmt.Errorf("%w: name is invalid", ErrInvalidInput)
	}
	if err := validateProducer(input.Producer); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidInput, err)
	}
	if input.Classification != evidence.ClassObserved && input.Classification != evidence.ClassPredicted {
		return fmt.Errorf("%w: classification must be observed or predicted", ErrInvalidInput)
	}
	if input.Completeness != evidence.CompletenessComplete && input.Completeness != evidence.CompletenessPartial {
		return fmt.Errorf("%w: completeness is invalid", ErrInvalidInput)
	}
	if err := evidence.ValidateRuntimeSignature(input.Runtime); err != nil {
		return fmt.Errorf("%w: runtime: %v", ErrInvalidInput, err)
	}
	if input.Classification == evidence.ClassObserved && input.Runtime.Origin != evidence.OriginObserved {
		return fmt.Errorf("%w: observed input requires an observed runtime", ErrClassification)
	}
	if input.Classification == evidence.ClassPredicted && input.Runtime.Origin != evidence.OriginDeclared {
		return fmt.Errorf("%w: predicted input requires a declared runtime", ErrClassification)
	}
	started, err := time.Parse(time.RFC3339Nano, input.StartedAt)
	if err != nil {
		return fmt.Errorf("%w: started_at must be RFC3339", ErrInvalidInput)
	}
	if input.FinishedAt == "" {
		if input.Completeness == evidence.CompletenessComplete {
			return fmt.Errorf("%w: complete input requires finished_at", ErrInvalidInput)
		}
	} else {
		finished, err := time.Parse(time.RFC3339Nano, input.FinishedAt)
		if err != nil || finished.Before(started) {
			return fmt.Errorf("%w: finished_at must be RFC3339 and not precede started_at", ErrInvalidInput)
		}
	}
	if input.Completeness == evidence.CompletenessComplete {
		unknown, err := evidence.UnknownDimensions(input.Runtime)
		if err != nil || len(unknown) != 0 {
			return fmt.Errorf("%w: complete input requires a complete runtime signature", ErrInvalidInput)
		}
	}
	if !digestPattern.MatchString(input.WorkloadDigest) || input.Attempt == 0 {
		return fmt.Errorf("%w: workload digest or attempt is invalid", ErrInvalidInput)
	}
	if len(input.Report) == 0 || len(input.Report) > MaxInputBytes {
		return fmt.Errorf("%w: report is empty or exceeds %d bytes", ErrInvalidInput, MaxInputBytes)
	}
	if _, err := strictjson.ReadOne(bytes.NewReader(input.Report), MaxInputBytes); err != nil {
		return fmt.Errorf("%w: report JSON: %w", ErrInvalidInput, err)
	}
	return nil
}

func ValidateNormalizedReport(report NormalizedReport) error {
	if report.Schema != NormalizedReportSchema || report.SchemaVersion != CurrentVersion {
		return fmt.Errorf("%w: unsupported schema or version", ErrInvalidReport)
	}
	if !namePattern.MatchString(report.Adapter.Name) || !revisionPattern.MatchString(report.Adapter.Revision) || !digestPattern.MatchString(report.InputDigest) {
		return fmt.Errorf("%w: invalid adapter identity or input digest", ErrInvalidReport)
	}
	if len(report.Originals) == 0 || len(report.Originals) != len(report.Mappings) || len(report.Originals) != len(report.Envelope.Metrics) {
		return fmt.Errorf("%w: originals, mappings, and envelope metrics must have equal non-zero lengths", ErrInvalidReport)
	}
	if err := evidence.ValidateEnvelope(report.Envelope); err != nil {
		return fmt.Errorf("%w: envelope: %v", ErrInvalidReport, err)
	}
	if report.Envelope.Source.Adapter != report.Adapter.Name || report.Envelope.Source.AdapterRevision != report.Adapter.Revision {
		return fmt.Errorf("%w: envelope adapter identity differs from report", ErrInvalidReport)
	}
	if len(report.Envelope.Artifacts) != 1 || report.Envelope.Artifacts[0].Name != "raw-report" || report.Envelope.Artifacts[0].Digest != report.InputDigest {
		return fmt.Errorf("%w: raw report artifact is not bound to input digest", ErrInvalidReport)
	}

	originals := make(map[string]OriginalMetric, len(report.Originals))
	for i, original := range report.Originals {
		if !namePattern.MatchString(original.Name) || !namePattern.MatchString(original.Definition) || original.Unit == "" ||
			math.IsNaN(original.Value) || math.IsInf(original.Value, 0) || original.SampleCount == 0 {
			return fmt.Errorf("%w: originals[%d] is invalid", ErrInvalidReport, i)
		}
		key := original.Name + "\x00" + original.Definition + "\x00" + original.Unit
		if _, exists := originals[key]; exists {
			return fmt.Errorf("%w: duplicate original metric %q", ErrInvalidReport, original.Name)
		}
		originals[key] = original
	}
	metrics := make(map[string]evidence.Metric, len(report.Envelope.Metrics))
	for _, metric := range report.Envelope.Metrics {
		metrics[metric.Name+"\x00"+metric.Semantics] = metric
	}
	seenMappings := make(map[string]struct{}, len(report.Mappings))
	for i, mapping := range report.Mappings {
		values := []float64{mapping.SourceValue, mapping.NormalizedValue, mapping.Scale, mapping.Offset}
		for _, value := range values {
			if math.IsNaN(value) || math.IsInf(value, 0) {
				return fmt.Errorf("%w: mappings[%d] contains a non-finite value", ErrInvalidReport, i)
			}
		}
		original, exists := originals[mapping.SourceName+"\x00"+mapping.SourceDefinition+"\x00"+mapping.SourceUnit]
		if !exists || original.Value != mapping.SourceValue {
			return fmt.Errorf("%w: mappings[%d] does not match an original metric", ErrInvalidReport, i)
		}
		metric, exists := metrics[mapping.NormalizedName+"\x00"+mapping.Semantics]
		if !exists || metric.Unit != mapping.NormalizedUnit || metric.Value != mapping.NormalizedValue || metric.SampleCount != original.SampleCount {
			return fmt.Errorf("%w: mappings[%d] does not match the normalized metric", ErrInvalidReport, i)
		}
		if mapping.NormalizedValue != mapping.SourceValue*mapping.Scale+mapping.Offset {
			return fmt.Errorf("%w: mappings[%d] conversion is inconsistent", ErrInvalidReport, i)
		}
		key := mapping.NormalizedName + "\x00" + mapping.Semantics
		if _, exists := seenMappings[key]; exists {
			return fmt.Errorf("%w: duplicate mapping target", ErrInvalidReport)
		}
		seenMappings[key] = struct{}{}
	}
	return nil
}

func validateProducer(producer ProducerIdentity) error {
	if !namePattern.MatchString(producer.Tool) || !validIdentifier(producer.ToolVersion) || !validIdentifier(producer.ReportSchema) {
		return errors.New("producer requires a valid tool, pinned version, and report schema")
	}
	for _, value := range []string{producer.ToolVersion, producer.ReportSchema} {
		if mutableVersion(value) {
			return errors.New("producer identity contains a mutable alias or version range")
		}
	}
	return nil
}

func validIdentifier(value string) bool {
	if value == "" || len(value) > 256 || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if character < 0x20 || character == 0x7f {
			return false
		}
	}
	return true
}

func mutableVersion(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "canary", "dev", "edge", "head", "latest", "main", "master", "nightly", "snapshot", "trunk", "unknown", "unstable":
		return true
	}
	return strings.ContainsAny(normalized, "*^~<>=") || strings.HasSuffix(normalized, ".x") ||
		strings.HasSuffix(normalized, "-snapshot") || strings.HasSuffix(normalized, ".snapshot")
}

func inputDigest(input []byte) string {
	digest := sha256.Sum256(input)
	return "sha256:" + hex.EncodeToString(digest[:])
}

// CanonicalJSON returns a stable normalized report representation.
func CanonicalJSON(report NormalizedReport) ([]byte, error) {
	if err := ValidateNormalizedReport(report); err != nil {
		return nil, err
	}
	canonical := report
	canonical.Originals = slices.Clone(report.Originals)
	slices.SortFunc(canonical.Originals, func(a, b OriginalMetric) int {
		return strings.Compare(a.Name+"\x00"+a.Definition+"\x00"+a.Unit, b.Name+"\x00"+b.Definition+"\x00"+b.Unit)
	})
	canonical.Mappings = slices.Clone(report.Mappings)
	slices.SortFunc(canonical.Mappings, func(a, b MappingRecord) int {
		return strings.Compare(a.NormalizedName+"\x00"+a.Semantics, b.NormalizedName+"\x00"+b.Semantics)
	})
	envelopeJSON, err := evidence.CanonicalJSON(report.Envelope)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(envelopeJSON, &canonical.Envelope); err != nil {
		return nil, fmt.Errorf("decode canonical evidence: %w", err)
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("encode canonical normalized report: %w", err)
	}
	return encoded, nil
}

func Digest(report NormalizedReport) (string, error) {
	canonical, err := CanonicalJSON(report)
	if err != nil {
		return "", err
	}
	return inputDigest(canonical), nil
}

func decodeStrict[T any](reader io.Reader, maxBytes int64, sentinel error) (T, error) {
	var value T
	data, err := strictjson.ReadOne(reader, maxBytes)
	if err != nil {
		return value, fmt.Errorf("%w: %w", sentinel, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, fmt.Errorf("%w: decode JSON: %v", sentinel, err)
	}
	return value, nil
}
