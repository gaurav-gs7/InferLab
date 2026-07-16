package evidence

import (
	"fmt"
	"regexp"
	"slices"
)

type CompatibilityStatus string

const (
	CompatibilityExact    CompatibilityStatus = "exact"
	CompatibilityByPolicy CompatibilityStatus = "compatible-by-policy"
	CompatibilityMismatch CompatibilityStatus = "incompatible"
	CompatibilityUnknown  CompatibilityStatus = "unknown"
)

// CompatibilityPolicy is an explicit, versioned exception to exact matching.
// It can ignore known differences but never unknown identity.
type CompatibilityPolicy struct {
	Name              string      `json:"name"`
	Version           string      `json:"version"`
	IgnoredDimensions []Dimension `json:"ignored_dimensions"`
}

// waivableDimensions is deliberately narrow. Driver patch equivalence can be
// established by an explicit, versioned policy; model, engine, container,
// accelerator, scheduler, and kernel identity are never generic waivers.
var waivableDimensions = map[Dimension]struct{}{
	DimensionDriverVersion: {},
}

type CompatibilityResult struct {
	Status             CompatibilityStatus `json:"status"`
	PolicyName         string              `json:"policy_name,omitempty"`
	PolicyVersion      string              `json:"policy_version,omitempty"`
	Differences        []Dimension         `json:"differences,omitempty"`
	IgnoredDifferences []Dimension         `json:"ignored_differences,omitempty"`
	UnknownDimensions  []Dimension         `json:"unknown_dimensions,omitempty"`
}

var policyVersionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+(?:\.[0-9]+)?$`)

// CompareRuntimeSignatures compares material identity with fail-closed precedence:
// an unignored mismatch is incompatible, then unknown identity is unknown.
func CompareRuntimeSignatures(left, right RuntimeSignature, policy *CompatibilityPolicy) (CompatibilityResult, error) {
	if err := ValidateRuntimeSignature(left); err != nil {
		return CompatibilityResult{}, err
	}
	if err := ValidateRuntimeSignature(right); err != nil {
		return CompatibilityResult{}, err
	}
	if err := validateCompatibilityPolicy(policy); err != nil {
		return CompatibilityResult{}, err
	}
	leftValues, err := dimensionValues(left)
	if err != nil {
		return CompatibilityResult{}, err
	}
	rightValues, err := dimensionValues(right)
	if err != nil {
		return CompatibilityResult{}, err
	}

	ignored := make(map[Dimension]struct{})
	result := CompatibilityResult{}
	if policy != nil {
		result.PolicyName = policy.Name
		result.PolicyVersion = policy.Version
		for _, dimension := range policy.IgnoredDimensions {
			ignored[dimension] = struct{}{}
		}
	}

	for _, dimension := range materialDimensions {
		leftValue, rightValue := leftValues[dimension], rightValues[dimension]
		if leftValue == "" || rightValue == "" {
			result.UnknownDimensions = append(result.UnknownDimensions, dimension)
			continue
		}
		if leftValue == rightValue {
			continue
		}
		if _, allowed := ignored[dimension]; allowed {
			result.IgnoredDifferences = append(result.IgnoredDifferences, dimension)
		} else {
			result.Differences = append(result.Differences, dimension)
		}
	}

	switch {
	case len(result.Differences) > 0:
		result.Status = CompatibilityMismatch
	case len(result.UnknownDimensions) > 0:
		result.Status = CompatibilityUnknown
	case len(result.IgnoredDifferences) > 0:
		result.Status = CompatibilityByPolicy
	default:
		result.Status = CompatibilityExact
	}
	return result, nil
}

func validateCompatibilityPolicy(policy *CompatibilityPolicy) error {
	if policy == nil {
		return nil
	}
	if !namePattern.MatchString(policy.Name) || !policyVersionPattern.MatchString(policy.Version) {
		return fmt.Errorf("%w: policy requires a valid name and semantic version", ErrInvalidPolicy)
	}
	known := make(map[Dimension]struct{}, len(materialDimensions))
	for _, dimension := range materialDimensions {
		known[dimension] = struct{}{}
	}
	seen := make(map[Dimension]struct{}, len(policy.IgnoredDimensions))
	for _, dimension := range policy.IgnoredDimensions {
		if _, exists := known[dimension]; !exists {
			return fmt.Errorf("%w: unknown dimension %q", ErrInvalidPolicy, dimension)
		}
		if _, allowed := waivableDimensions[dimension]; !allowed {
			return fmt.Errorf("%w: dimension %q is not safely waivable in v1", ErrInvalidPolicy, dimension)
		}
		if _, exists := seen[dimension]; exists {
			return fmt.Errorf("%w: duplicate dimension %q", ErrInvalidPolicy, dimension)
		}
		seen[dimension] = struct{}{}
	}
	if !slices.IsSorted(policy.IgnoredDimensions) {
		return fmt.Errorf("%w: ignored_dimensions must be sorted", ErrInvalidPolicy)
	}
	return nil
}
