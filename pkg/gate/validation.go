package gate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"slices"
	"time"

	"github.com/gaurav-gs7/InferLab/internal/strictjson"
	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

const (
	maxRegions             = 128
	maxRules               = 256
	maxEvidence            = 512
	maxEvaluationMetrics   = 65536
	maxEvaluationArtifacts = 4096
)

var (
	namePattern   = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,126}[a-z0-9])?$`)
	digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

func ValidateEvaluation(evaluation Evaluation) error {
	if evaluation.Schema != EvaluationSchema || evaluation.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("%w: unsupported schema or version", ErrInvalidEvaluation)
	}
	if !namePattern.MatchString(evaluation.Name) || !digestPattern.MatchString(evaluation.ChangeDigest) {
		return fmt.Errorf("%w: name or change_digest is invalid", ErrInvalidEvaluation)
	}
	if _, err := time.Parse(time.RFC3339Nano, evaluation.EvaluatedAt); err != nil {
		return fmt.Errorf("%w: evaluated_at must be RFC3339: %v", ErrInvalidEvaluation, err)
	}
	if evaluation.MaxEvidenceAgeSeconds == 0 || evaluation.MaxEvidenceAgeSeconds > 90*24*60*60 {
		return fmt.Errorf("%w: max_evidence_age_seconds must be in [1,7776000]", ErrInvalidEvaluation)
	}
	if math.Abs(evaluation.ConfidenceLevel-0.95) > 1e-12 {
		return fmt.Errorf("%w: v1 supports confidence_level 0.95", ErrInvalidEvaluation)
	}
	if !finite(evaluation.MaximumViolationProbability) || evaluation.MaximumViolationProbability <= 0 || evaluation.MaximumViolationProbability > 0.5 {
		return fmt.Errorf("%w: maximum_violation_probability must be in (0,0.5]", ErrInvalidEvaluation)
	}
	if err := evidence.ValidateRuntimeSignature(evaluation.TargetRuntime); err != nil {
		return fmt.Errorf("%w: target_runtime: %v", ErrInvalidEvaluation, err)
	}
	unknown, err := evidence.UnknownDimensions(evaluation.TargetRuntime)
	if err != nil {
		return fmt.Errorf("%w: target_runtime: %v", ErrInvalidEvaluation, err)
	}
	if len(unknown) > 0 {
		return fmt.Errorf("%w: target_runtime has unknown dimensions: %v", ErrInvalidEvaluation, unknown)
	}
	if _, err := evidence.CompareRuntimeSignatures(evaluation.TargetRuntime, evaluation.TargetRuntime, evaluation.CompatibilityPolicy); err != nil {
		return fmt.Errorf("%w: compatibility_policy: %v", ErrInvalidEvaluation, err)
	}

	if len(evaluation.Regions) == 0 || len(evaluation.Regions) > maxRegions {
		return fmt.Errorf("%w: regions must contain 1..%d entries", ErrInvalidEvaluation, maxRegions)
	}
	regions := make(map[string]Region, len(evaluation.Regions))
	for i, region := range evaluation.Regions {
		if !namePattern.MatchString(region.Name) || !digestPattern.MatchString(region.WorkloadDigest) {
			return fmt.Errorf("%w: regions[%d] has invalid identity", ErrInvalidEvaluation, i)
		}
		if _, exists := regions[region.Name]; exists {
			return fmt.Errorf("%w: duplicate region %q", ErrInvalidEvaluation, region.Name)
		}
		if err := validateBounds(region.Minimum, region.Maximum); err != nil {
			return fmt.Errorf("%w: regions[%d]: %v", ErrInvalidEvaluation, i, err)
		}
		if err := ValidateFaultPoint(region.Fault); err != nil {
			return fmt.Errorf("%w: regions[%d]: %w", ErrInvalidEvaluation, i, err)
		}
		regions[region.Name] = region
	}

	if len(evaluation.Rules) == 0 || len(evaluation.Rules) > maxRules {
		return fmt.Errorf("%w: rules must contain 1..%d entries", ErrInvalidEvaluation, maxRules)
	}
	seenRules := make(map[string]struct{}, len(evaluation.Rules))
	for i, rule := range evaluation.Rules {
		if !namePattern.MatchString(rule.ID) {
			return fmt.Errorf("%w: rules[%d].id is invalid", ErrInvalidEvaluation, i)
		}
		if _, exists := seenRules[rule.ID]; exists {
			return fmt.Errorf("%w: duplicate rule %q", ErrInvalidEvaluation, rule.ID)
		}
		seenRules[rule.ID] = struct{}{}
		if _, exists := regions[rule.Region]; !exists {
			return fmt.Errorf("%w: rules[%d] references unknown region %q", ErrInvalidEvaluation, i, rule.Region)
		}
		contract, supported := metricCatalog[rule.Metric]
		if !supported {
			return fmt.Errorf("%w: rules[%d] metric %q: %w", ErrInvalidEvaluation, i, rule.Metric, ErrUnsupportedMetric)
		}
		if rule.Semantics != contract.semantics || rule.Unit != contract.unit || rule.Direction != contract.direction {
			return fmt.Errorf("%w: rules[%d] changes the declared contract for %q", ErrInvalidEvaluation, i, rule.Metric)
		}
		if !finite(rule.Threshold) || rule.Threshold < 0 || rule.MinimumSamples == 0 {
			return fmt.Errorf("%w: rules[%d] has invalid threshold or minimum_samples", ErrInvalidEvaluation, i)
		}
		if rule.Unit == "ratio" && rule.Threshold > 1 {
			return fmt.Errorf("%w: rules[%d] ratio threshold must be in [0,1]", ErrInvalidEvaluation, i)
		}
	}

	if len(evaluation.Evidence) > maxEvidence {
		return fmt.Errorf("%w: evidence has %d entries, maximum is %d", ErrInvalidEvaluation, len(evaluation.Evidence), maxEvidence)
	}
	seenEvidence := make(map[string]struct{}, len(evaluation.Evidence))
	totalMetrics := 0
	totalArtifacts := 0
	for i, node := range evaluation.Evidence {
		if err := validateEvidenceNode(node); err != nil {
			return fmt.Errorf("%w: evidence[%d]: %v", ErrInvalidEvaluation, i, err)
		}
		digest, err := evidence.Digest(node.Envelope)
		if err != nil {
			return fmt.Errorf("%w: evidence[%d] digest: %v", ErrInvalidEvaluation, i, err)
		}
		if _, exists := seenEvidence[digest]; exists {
			return fmt.Errorf("%w: duplicate evidence digest %s", ErrInvalidEvaluation, digest)
		}
		seenEvidence[digest] = struct{}{}
		totalMetrics += len(node.Envelope.Metrics)
		totalArtifacts += len(node.Envelope.Artifacts)
		if totalMetrics > maxEvaluationMetrics || totalArtifacts > maxEvaluationArtifacts {
			return fmt.Errorf("%w: aggregate evidence exceeds %d metrics or %d artifacts", ErrInvalidEvaluation, maxEvaluationMetrics, maxEvaluationArtifacts)
		}
	}
	return nil
}

func validateBounds(minimum, maximum LoadShape) error {
	values := []struct {
		name string
		min  uint64
		max  uint64
	}{
		{name: "concurrency", min: uint64(minimum.Concurrency), max: uint64(maximum.Concurrency)},
		{name: "prompt_tokens", min: minimum.PromptTokens, max: maximum.PromptTokens},
		{name: "output_tokens", min: minimum.OutputTokens, max: maximum.OutputTokens},
		{name: "tenant_count", min: uint64(minimum.TenantCount), max: uint64(maximum.TenantCount)},
	}
	for _, value := range values {
		if value.min == 0 || value.max < value.min {
			return fmt.Errorf("%s bounds must be positive and ordered", value.name)
		}
	}
	if !finite(minimum.ArrivalRate) || !finite(maximum.ArrivalRate) || minimum.ArrivalRate <= 0 || maximum.ArrivalRate < minimum.ArrivalRate {
		return errors.New("arrival_rate_per_second bounds must be finite, positive, and ordered")
	}
	return nil
}

func validateLoadShape(shape LoadShape) error {
	if shape.Concurrency == 0 || shape.PromptTokens == 0 || shape.OutputTokens == 0 || shape.TenantCount == 0 || !finite(shape.ArrivalRate) || shape.ArrivalRate <= 0 {
		return errors.New("workload dimensions must be finite and positive")
	}
	return nil
}

func ValidateFaultPoint(fault FaultPoint) error {
	if !finite(fault.LongContextFraction) {
		return fmt.Errorf("%w: long_context_fraction must be finite", ErrUnsupportedFault)
	}
	switch fault.Type {
	case FaultNone:
		if fault.LostReplicas != 0 || fault.DurationSeconds != 0 || fault.LongContextTokens != 0 || fault.LongContextFraction != 0 {
			return fmt.Errorf("%w: none fault must not contain parameters", ErrUnsupportedFault)
		}
	case FaultReplicaLoss:
		if fault.LostReplicas != 1 || fault.DurationSeconds == 0 || fault.DurationSeconds > 600 || fault.LongContextTokens != 0 || fault.LongContextFraction != 0 {
			return fmt.Errorf("%w: replica-loss requires one lost replica and duration in [1,600] seconds", ErrUnsupportedFault)
		}
	case FaultLongContextSpike:
		if fault.LostReplicas != 0 || fault.DurationSeconds != 0 || fault.LongContextTokens < 4096 || fault.LongContextTokens > 262144 || fault.LongContextFraction < 0.01 || fault.LongContextFraction > 1 || math.Abs(fault.LongContextFraction*100-math.Round(fault.LongContextFraction*100)) > 1e-9 {
			return fmt.Errorf("%w: long-context-spike requires 4096..262144 tokens and fraction in 0.01 increments within [0.01,1]", ErrUnsupportedFault)
		}
	default:
		return fmt.Errorf("%w: %q; supported types are none, replica-loss, and long-context-spike", ErrUnsupportedFault, fault.Type)
	}
	return nil
}

func validateEvidenceNode(node EvidenceNode) error {
	if err := evidence.ValidateEnvelope(node.Envelope); err != nil {
		return err
	}
	if err := validateLoadShape(node.Workload.LoadShape); err != nil {
		return err
	}
	if err := ValidateFaultPoint(node.Workload.Fault); err != nil {
		return err
	}
	metrics := make(map[string]evidence.Metric, len(node.Envelope.Metrics))
	for _, metric := range node.Envelope.Metrics {
		metrics[metricKey(metric.Name, metric.Semantics)] = metric
	}
	seen := make(map[string]struct{}, len(node.Uncertainties))
	for i, uncertainty := range node.Uncertainties {
		key := metricKey(uncertainty.Name, uncertainty.Semantics)
		metric, exists := metrics[key]
		if !exists {
			return fmt.Errorf("uncertainties[%d] references a missing metric", i)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("uncertainties[%d] duplicates a metric", i)
		}
		seen[key] = struct{}{}
		switch uncertainty.Method {
		case UncertaintyBootstrap, UncertaintySample:
			if metric.Name == MetricRequestGoodput && metric.Semantics == metricCatalog[MetricRequestGoodput].semantics {
				return fmt.Errorf("uncertainties[%d] request goodput requires binomial successes and trials", i)
			}
			if uncertainty.StandardError == nil || !finite(*uncertainty.StandardError) || *uncertainty.StandardError < 0 || uncertainty.Successes != nil || uncertainty.Trials != nil {
				return fmt.Errorf("uncertainties[%d] requires only a non-negative standard_error", i)
			}
		case UncertaintyBinomial:
			if uncertainty.StandardError != nil || uncertainty.Successes == nil || uncertainty.Trials == nil || *uncertainty.Trials == 0 || *uncertainty.Successes > *uncertainty.Trials {
				return fmt.Errorf("uncertainties[%d] requires valid successes and trials", i)
			}
			if metric.Unit != "ratio" && metric.Unit != "probability" {
				return fmt.Errorf("uncertainties[%d] binomial method requires ratio or probability", i)
			}
			if metric.Name != MetricRequestGoodput || metric.Semantics != metricCatalog[MetricRequestGoodput].semantics {
				return fmt.Errorf("uncertainties[%d] binomial method is supported only for request goodput", i)
			}
			if *uncertainty.Trials != metric.SampleCount {
				return fmt.Errorf("uncertainties[%d] trials must equal metric sample_count", i)
			}
			ratio := float64(*uncertainty.Successes) / float64(*uncertainty.Trials)
			if math.Abs(ratio-metric.Value) > 1e-12 {
				return fmt.Errorf("uncertainties[%d] successes/trials differs from metric value", i)
			}
		default:
			return fmt.Errorf("uncertainties[%d] has unsupported method %q", i, uncertainty.Method)
		}
	}
	return nil
}

func metricKey(name, semantics string) string { return name + "\x00" + semantics }
func finite(value float64) bool               { return !math.IsNaN(value) && !math.IsInf(value, 0) }

func pointInRegion(point WorkloadPoint, region Region) bool {
	return point.Fault == region.Fault &&
		point.Concurrency >= region.Minimum.Concurrency && point.Concurrency <= region.Maximum.Concurrency &&
		point.PromptTokens >= region.Minimum.PromptTokens && point.PromptTokens <= region.Maximum.PromptTokens &&
		point.OutputTokens >= region.Minimum.OutputTokens && point.OutputTokens <= region.Maximum.OutputTokens &&
		point.TenantCount >= region.Minimum.TenantCount && point.TenantCount <= region.Maximum.TenantCount &&
		point.ArrivalRate >= region.Minimum.ArrivalRate && point.ArrivalRate <= region.Maximum.ArrivalRate
}

func sortedUniqueCodes(codes []FindingCode) []FindingCode {
	result := slices.Clone(codes)
	slices.Sort(result)
	return slices.Compact(result)
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
