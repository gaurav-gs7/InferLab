package change

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxNameBytes       = 63
	maxIdentifierBytes = 256
	maxTenants         = 128
	maxFaults          = 16
	maxFaultPoints     = 64
)

var (
	documentNamePattern  = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)
	imageDigestPattern   = regexp.MustCompile(`^[^[:space:]@]+@sha256:[0-9a-f]{64}$`)
	modelRevisionPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
)

// Validate checks the v0.1 syntax, safety invariants, and support envelope.
func Validate(document Document) error {
	if document.Schema != Schema {
		return fmt.Errorf("%w: %q", ErrUnsupportedSchema, document.Schema)
	}
	if document.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("%w: %q", ErrUnsupportedVersion, document.SchemaVersion)
	}
	if !documentNamePattern.MatchString(document.Name) {
		return fmt.Errorf("%w: name must be a lowercase DNS label of at most %d bytes", ErrInvalidDocument, maxNameBytes)
	}
	if err := validateConfiguration("baseline", document.Baseline); err != nil {
		return err
	}
	if err := validateConfiguration("candidate", document.Candidate); err != nil {
		return err
	}
	if reflect.DeepEqual(document.Baseline, document.Candidate) {
		return fmt.Errorf("%w: candidate must differ from baseline", ErrInvalidDocument)
	}
	if err := validateWorkload(document.Workload); err != nil {
		return err
	}
	if err := validatePolicies(document.Policies); err != nil {
		return err
	}
	if len(document.Faults) > maxFaults {
		return fmt.Errorf("%w: faults has %d entries, maximum is %d", ErrInvalidDocument, len(document.Faults), maxFaults)
	}
	seenFaults := make(map[string]struct{}, len(document.Faults))
	for i, fault := range document.Faults {
		if _, exists := seenFaults[fault.Type]; exists {
			return fmt.Errorf("%w: fault %d repeats type %q", ErrInvalidDocument, i, fault.Type)
		}
		seenFaults[fault.Type] = struct{}{}
		if err := validateFault(i, fault); err != nil {
			return err
		}
	}
	if !positiveFinite(document.Budget.MaximumExperimentCostUSD) {
		return fmt.Errorf("%w: budget.maximum_experiment_cost_usd must be finite and greater than zero", ErrInvalidDocument)
	}
	if document.Budget.MaximumGPUMinutes == 0 {
		return fmt.Errorf("%w: budget.maximum_gpu_minutes must be greater than zero", ErrInvalidDocument)
	}
	return nil
}

func validateConfiguration(path string, configuration Configuration) error {
	if configuration.Engine.Name != "vllm" {
		return fmt.Errorf("%w: %s.engine.name %q; v0.1 supports only vllm", ErrUnsupportedFeature, path, configuration.Engine.Name)
	}
	if !imageDigestPattern.MatchString(configuration.Engine.Image) {
		return fmt.Errorf("%w: %s.engine.image must include an immutable sha256 digest", ErrInvalidDocument, path)
	}
	if err := validateIdentifier(path+".model.id", configuration.Model.ID); err != nil {
		return err
	}
	if err := validateIdentifier(path+".model.revision", configuration.Model.Revision); err != nil {
		return err
	}
	if !modelRevisionPattern.MatchString(configuration.Model.Revision) {
		return fmt.Errorf("%w: %s.model.revision must be an immutable 40- or 64-character lowercase hexadecimal revision", ErrInvalidDocument, path)
	}
	switch configuration.Model.Quantization {
	case "none", "awq", "gptq":
	default:
		return fmt.Errorf("%w: %s.model.quantization %q; supported values are none, awq, and gptq", ErrUnsupportedFeature, path, configuration.Model.Quantization)
	}
	if configuration.Hardware.Accelerator != "nvidia-l4" || configuration.Hardware.InstanceType != "g6.xlarge" {
		return fmt.Errorf("%w: %s.hardware must be nvidia-l4 on g6.xlarge", ErrUnsupportedFeature, path)
	}
	if configuration.Hardware.Replicas == 0 || configuration.Hardware.Replicas > 2 {
		return fmt.Errorf("%w: %s.hardware.replicas must be between 1 and 2", ErrUnsupportedFeature, path)
	}
	if configuration.Scheduler.MaxNumBatchedTokens == 0 {
		return fmt.Errorf("%w: %s.scheduler.max_num_batched_tokens must be greater than zero", ErrInvalidDocument, path)
	}
	if configuration.Scheduler.MaxSequences == 0 {
		return fmt.Errorf("%w: %s.scheduler.max_sequences must be greater than zero", ErrInvalidDocument, path)
	}
	return nil
}

func validateWorkload(workload Workload) error {
	parsed, err := url.Parse(workload.Trace)
	if err != nil || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("%w: workload.trace must be a credential-free file:// or s3:// URI", ErrInvalidDocument)
	}
	switch parsed.Scheme {
	case "s3":
		if parsed.Host == "" || strings.Trim(parsed.Path, "/") == "" {
			return fmt.Errorf("%w: workload.trace s3 URI requires bucket and object key", ErrInvalidDocument)
		}
	case "file":
		if parsed.Host != "" || !strings.HasPrefix(parsed.Path, "/") {
			return fmt.Errorf("%w: workload.trace file URI must use an absolute local path", ErrInvalidDocument)
		}
	default:
		return fmt.Errorf("%w: workload.trace scheme %q; supported schemes are file and s3", ErrUnsupportedFeature, parsed.Scheme)
	}
	if !positiveFinite(workload.ReplaySpeed) || workload.ReplaySpeed > 100 {
		return fmt.Errorf("%w: workload.replay_speed must be finite and in the range (0, 100]", ErrInvalidDocument)
	}
	if len(workload.Tenants) == 0 || len(workload.Tenants) > maxTenants {
		return fmt.Errorf("%w: workload.tenants must contain 1..%d entries", ErrInvalidDocument, maxTenants)
	}
	seen := make(map[string]struct{}, len(workload.Tenants))
	for i, tenant := range workload.Tenants {
		if err := validateIdentifier(fmt.Sprintf("workload.tenants[%d].name", i), tenant.Name); err != nil {
			return err
		}
		if tenant.Weight == 0 {
			return fmt.Errorf("%w: workload.tenants[%d].weight must be greater than zero", ErrInvalidDocument, i)
		}
		if _, exists := seen[tenant.Name]; exists {
			return fmt.Errorf("%w: workload.tenants repeats name %q", ErrInvalidDocument, tenant.Name)
		}
		seen[tenant.Name] = struct{}{}
	}
	return nil
}

func validatePolicies(policies Policies) error {
	if policies.TTFTP99Milliseconds == 0 {
		return fmt.Errorf("%w: policies.ttft_p99_milliseconds must be greater than zero", ErrInvalidDocument)
	}
	if policies.TPOTP99Milliseconds == 0 {
		return fmt.Errorf("%w: policies.tpot_p99_milliseconds must be greater than zero", ErrInvalidDocument)
	}
	if !unitInterval(policies.MinimumFairnessIndex) || policies.MinimumFairnessIndex == 0 {
		return fmt.Errorf("%w: policies.minimum_fairness_index must be in the range (0, 1]", ErrInvalidDocument)
	}
	if !positiveFinite(policies.MaximumCostPerMillionTokensUSD) {
		return fmt.Errorf("%w: policies.maximum_cost_per_million_tokens_usd must be finite and greater than zero", ErrInvalidDocument)
	}
	if !unitInterval(policies.MaximumViolationProbability) || policies.MaximumViolationProbability == 0 || policies.MaximumViolationProbability > 0.5 {
		return fmt.Errorf("%w: policies.maximum_violation_probability must be in the range (0, 0.5]", ErrInvalidDocument)
	}
	return nil
}

func validateFault(index int, fault Fault) error {
	switch fault.Type {
	case "replica-loss":
		if !unitInterval(fault.Probability) || fault.Probability == 0 {
			return fmt.Errorf("%w: faults[%d].probability must be in the range (0, 1]", ErrInvalidDocument, index)
		}
		if err := validateIncreasingUint32(fmt.Sprintf("faults[%d].duration_seconds", index), fault.DurationSeconds); err != nil {
			return err
		}
		if len(fault.PromptTokens) != 0 {
			return fmt.Errorf("%w: faults[%d].prompt_tokens is not valid for replica-loss", ErrInvalidDocument, index)
		}
	case "long-context-spike":
		if fault.Probability != 0 {
			return fmt.Errorf("%w: faults[%d].probability is not valid for long-context-spike", ErrInvalidDocument, index)
		}
		if len(fault.DurationSeconds) != 0 {
			return fmt.Errorf("%w: faults[%d].duration_seconds is not valid for long-context-spike", ErrInvalidDocument, index)
		}
		if err := validateIncreasingUint64(fmt.Sprintf("faults[%d].prompt_tokens", index), fault.PromptTokens); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%w: faults[%d].type %q; v0.1 supports replica-loss and long-context-spike", ErrUnsupportedFeature, index, fault.Type)
	}
	return nil
}

func validateIncreasingUint32(path string, values []uint32) error {
	if len(values) == 0 || len(values) > maxFaultPoints {
		return fmt.Errorf("%w: %s must contain 1..%d points", ErrInvalidDocument, path, maxFaultPoints)
	}
	for i, value := range values {
		if value == 0 || (i > 0 && value <= values[i-1]) {
			return fmt.Errorf("%w: %s must be positive and strictly increasing", ErrInvalidDocument, path)
		}
	}
	return nil
}

func validateIncreasingUint64(path string, values []uint64) error {
	if len(values) == 0 || len(values) > maxFaultPoints {
		return fmt.Errorf("%w: %s must contain 1..%d points", ErrInvalidDocument, path, maxFaultPoints)
	}
	for i, value := range values {
		if value == 0 || (i > 0 && value <= values[i-1]) {
			return fmt.Errorf("%w: %s must be positive and strictly increasing", ErrInvalidDocument, path)
		}
	}
	return nil
}

func validateIdentifier(path, value string) error {
	if value == "" || len(value) > maxIdentifierBytes || !utf8.ValidString(value) {
		return fmt.Errorf("%w: %s must be valid UTF-8 and contain 1..%d bytes", ErrInvalidDocument, path, maxIdentifierBytes)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: %s contains control characters", ErrInvalidDocument, path)
		}
	}
	return nil
}

func positiveFinite(value float64) bool {
	return value > 0 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

func unitInterval(value float64) bool {
	return value >= 0 && value <= 1 && !math.IsNaN(value) && !math.IsInf(value, 0)
}

// canonicalCopy normalizes semantically unordered collections before hashing.
func canonicalCopy(document Document) Document {
	canonical := document
	canonical.Workload.Tenants = slices.Clone(document.Workload.Tenants)
	slices.SortFunc(canonical.Workload.Tenants, func(a, b Tenant) int {
		return strings.Compare(a.Name, b.Name)
	})
	canonical.Faults = slices.Clone(document.Faults)
	slices.SortFunc(canonical.Faults, func(a, b Fault) int {
		if byType := strings.Compare(a.Type, b.Type); byType != 0 {
			return byType
		}
		aJSON, _ := json.Marshal(a)
		bJSON, _ := json.Marshal(b)
		return strings.Compare(string(aJSON), string(bJSON))
	})
	return canonical
}
