package evidence

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
)

const (
	maxMetrics   = 4096
	maxArtifacts = 256
	maxParents   = 256
)

var mediaTypePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9!#$&^_.+-]*/[a-z0-9][a-z0-9!#$&^_.+-]*$`)

var supportedUnits = map[string]struct{}{
	"count":             {},
	"milliseconds":      {},
	"probability":       {},
	"ratio":             {},
	"seconds":           {},
	"tokens":            {},
	"tokens_per_second": {},
	"usd":               {},
}

// Decode reads one bounded evidence envelope, rejecting unknown fields and
// trailing values before validation.
func Decode(reader io.Reader) (Envelope, error) {
	if reader == nil {
		return Envelope{}, fmt.Errorf("%w: reader is nil", ErrInvalidEnvelope)
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxDocumentBytes+1))
	if err != nil {
		return Envelope{}, fmt.Errorf("read evidence envelope: %w", err)
	}
	if len(data) > MaxDocumentBytes {
		return Envelope{}, ErrDocumentTooLarge
	}
	if err := validateJSONShape(data, ErrInvalidEnvelope); err != nil {
		return Envelope{}, err
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var envelope Envelope
	if err := decoder.Decode(&envelope); err != nil {
		return Envelope{}, fmt.Errorf("%w: decode JSON: %v", ErrInvalidEnvelope, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Envelope{}, fmt.Errorf("%w: trailing JSON value", ErrInvalidEnvelope)
		}
		return Envelope{}, fmt.Errorf("%w: trailing content: %v", ErrInvalidEnvelope, err)
	}
	if err := ValidateEnvelope(envelope); err != nil {
		return Envelope{}, err
	}
	return envelope, nil
}

// ValidateEnvelope checks syntax, provenance, completeness, and bounds.
func ValidateEnvelope(envelope Envelope) error {
	if envelope.Schema != EnvelopeSchema {
		return fmt.Errorf("%w: %q", ErrUnsupportedSchema, envelope.Schema)
	}
	if envelope.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("%w: %q", ErrUnsupportedVersion, envelope.SchemaVersion)
	}
	if !namePattern.MatchString(envelope.Name) {
		return fmt.Errorf("%w: name is invalid", ErrInvalidEnvelope)
	}
	switch envelope.Classification {
	case ClassObserved, ClassPredicted, ClassDerived, ClassAsserted:
	default:
		return fmt.Errorf("%w: classification is invalid", ErrInvalidEnvelope)
	}
	if envelope.Completeness != CompletenessComplete && envelope.Completeness != CompletenessPartial {
		return fmt.Errorf("%w: completeness must be complete or partial", ErrInvalidEnvelope)
	}
	if err := validateSource(envelope.Source); err != nil {
		return err
	}
	if err := ValidateRuntimeSignature(envelope.Runtime); err != nil {
		return err
	}
	if envelope.Classification == ClassObserved && envelope.Runtime.Origin != OriginObserved {
		return fmt.Errorf("%w: observed evidence requires an observed runtime signature", ErrInvalidEnvelope)
	}
	unknown, err := UnknownDimensions(envelope.Runtime)
	if err != nil {
		return err
	}
	if envelope.Completeness == CompletenessComplete && len(unknown) > 0 {
		return fmt.Errorf("%w: runtime signature has unknown dimensions: %v", ErrIncompleteEvidence, unknown)
	}
	if !digestPattern.MatchString(envelope.WorkloadDigest) {
		return fmt.Errorf("%w: workload_digest must be a sha256 digest", ErrInvalidEnvelope)
	}
	if envelope.Attempt == 0 {
		return fmt.Errorf("%w: attempt must be greater than zero", ErrInvalidEnvelope)
	}
	started, err := time.Parse(time.RFC3339Nano, envelope.StartedAt)
	if err != nil {
		return fmt.Errorf("%w: started_at must be RFC3339: %v", ErrInvalidEnvelope, err)
	}
	if envelope.FinishedAt == "" {
		if envelope.Completeness == CompletenessComplete {
			return fmt.Errorf("%w: complete evidence requires finished_at", ErrIncompleteEvidence)
		}
	} else {
		finished, err := time.Parse(time.RFC3339Nano, envelope.FinishedAt)
		if err != nil {
			return fmt.Errorf("%w: finished_at must be RFC3339: %v", ErrInvalidEnvelope, err)
		}
		if finished.Before(started) {
			return fmt.Errorf("%w: finished_at precedes started_at", ErrInvalidEnvelope)
		}
	}
	if len(envelope.Metrics) > maxMetrics {
		return fmt.Errorf("%w: metrics has %d entries, maximum is %d", ErrInvalidEnvelope, len(envelope.Metrics), maxMetrics)
	}
	if envelope.Completeness == CompletenessComplete && len(envelope.Metrics) == 0 {
		return fmt.Errorf("%w: complete evidence requires metrics", ErrIncompleteEvidence)
	}
	seenMetrics := make(map[string]struct{}, len(envelope.Metrics))
	for i, metric := range envelope.Metrics {
		if !namePattern.MatchString(metric.Name) || !namePattern.MatchString(metric.Semantics) {
			return fmt.Errorf("%w: metrics[%d] has invalid name or semantics", ErrInvalidEnvelope, i)
		}
		if math.IsNaN(metric.Value) || math.IsInf(metric.Value, 0) {
			return fmt.Errorf("%w: metrics[%d].value must be finite", ErrInvalidEnvelope, i)
		}
		if _, supported := supportedUnits[metric.Unit]; !supported {
			return fmt.Errorf("%w: metrics[%d].unit %q is unsupported", ErrInvalidEnvelope, i, metric.Unit)
		}
		if metric.Unit == "probability" && (metric.Value < 0 || metric.Value > 1) {
			return fmt.Errorf("%w: metrics[%d] probability must be in [0,1]", ErrInvalidEnvelope, i)
		}
		if metric.SampleCount == 0 {
			return fmt.Errorf("%w: metrics[%d].sample_count must be greater than zero", ErrInvalidEnvelope, i)
		}
		key := metric.Name + "\x00" + metric.Semantics
		if _, exists := seenMetrics[key]; exists {
			return fmt.Errorf("%w: duplicate metric name/semantics %q/%q", ErrInvalidEnvelope, metric.Name, metric.Semantics)
		}
		seenMetrics[key] = struct{}{}
	}
	if len(envelope.Artifacts) == 0 || len(envelope.Artifacts) > maxArtifacts {
		return fmt.Errorf("%w: artifacts must contain 1..%d entries", ErrInvalidEnvelope, maxArtifacts)
	}
	seenArtifacts := make(map[string]struct{}, len(envelope.Artifacts))
	for i, artifact := range envelope.Artifacts {
		if !namePattern.MatchString(artifact.Name) {
			return fmt.Errorf("%w: artifacts[%d].name is invalid", ErrInvalidEnvelope, i)
		}
		if !mediaTypePattern.MatchString(artifact.MediaType) {
			return fmt.Errorf("%w: artifacts[%d].media_type is invalid", ErrInvalidEnvelope, i)
		}
		if !digestPattern.MatchString(artifact.Digest) {
			return fmt.Errorf("%w: artifacts[%d].digest must be a sha256 digest", ErrInvalidEnvelope, i)
		}
		if _, exists := seenArtifacts[artifact.Name]; exists {
			return fmt.Errorf("%w: artifacts repeats name %q", ErrInvalidEnvelope, artifact.Name)
		}
		seenArtifacts[artifact.Name] = struct{}{}
	}
	if len(envelope.ParentDigests) > maxParents {
		return fmt.Errorf("%w: parent_digests has %d entries, maximum is %d", ErrInvalidEnvelope, len(envelope.ParentDigests), maxParents)
	}
	seenParents := make(map[string]struct{}, len(envelope.ParentDigests))
	for i, digest := range envelope.ParentDigests {
		if !digestPattern.MatchString(digest) {
			return fmt.Errorf("%w: parent_digests[%d] must be a sha256 digest", ErrInvalidEnvelope, i)
		}
		if _, exists := seenParents[digest]; exists {
			return fmt.Errorf("%w: duplicate parent digest %q", ErrInvalidEnvelope, digest)
		}
		seenParents[digest] = struct{}{}
	}
	if envelope.Classification == ClassDerived {
		if len(envelope.ParentDigests) == 0 || envelope.Transformation == nil {
			return fmt.Errorf("%w: derived evidence requires parents and transformation", ErrInvalidEnvelope)
		}
	}
	if envelope.Transformation != nil {
		if !namePattern.MatchString(envelope.Transformation.Name) || !revisionPattern.MatchString(envelope.Transformation.Revision) {
			return fmt.Errorf("%w: transformation requires a valid name and immutable revision", ErrInvalidEnvelope)
		}
	}
	return nil
}

func validateSource(source Source) error {
	fields := []struct {
		path  string
		value string
	}{
		{path: "source.tool", value: source.Tool},
		{path: "source.tool_version", value: source.ToolVersion},
		{path: "source.report_schema", value: source.ReportSchema},
		{path: "source.adapter", value: source.Adapter},
	}
	for _, field := range fields {
		if err := validateRequiredIdentifier(field.path, field.value, ErrInvalidEnvelope); err != nil {
			return err
		}
	}
	if isMutableAlias(source.ToolVersion) || isMutableAlias(source.ReportSchema) {
		return fmt.Errorf("%w: source tool version and report schema must be pinned", ErrInvalidEnvelope)
	}
	if !namePattern.MatchString(source.Tool) || !namePattern.MatchString(source.Adapter) {
		return fmt.Errorf("%w: source tool and adapter names are invalid", ErrInvalidEnvelope)
	}
	if !revisionPattern.MatchString(source.AdapterRevision) {
		return fmt.Errorf("%w: source.adapter_revision must be immutable", ErrInvalidEnvelope)
	}
	return nil
}

// CanonicalJSON returns the stable representation used for evidence identity.
func CanonicalJSON(envelope Envelope) ([]byte, error) {
	if err := ValidateEnvelope(envelope); err != nil {
		return nil, err
	}
	canonical := envelope
	canonical.Runtime = canonicalSignature(envelope.Runtime)
	canonical.Metrics = slices.Clone(envelope.Metrics)
	slices.SortFunc(canonical.Metrics, func(a, b Metric) int {
		if byName := strings.Compare(a.Name, b.Name); byName != 0 {
			return byName
		}
		return strings.Compare(a.Semantics, b.Semantics)
	})
	canonical.Artifacts = slices.Clone(envelope.Artifacts)
	slices.SortFunc(canonical.Artifacts, func(a, b Artifact) int {
		return strings.Compare(a.Name, b.Name)
	})
	canonical.ParentDigests = slices.Clone(envelope.ParentDigests)
	slices.Sort(canonical.ParentDigests)
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("encode canonical evidence envelope: %w", err)
	}
	return encoded, nil
}

// Digest returns a sha256-prefixed digest of CanonicalJSON.
func Digest(envelope Envelope) (string, error) {
	canonical, err := CanonicalJSON(envelope)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
