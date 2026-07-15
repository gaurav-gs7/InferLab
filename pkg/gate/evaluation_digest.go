package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

// CanonicalEvaluationJSON binds workload coordinates and uncertainty metadata
// as well as envelope digests. Semantically unordered collections are sorted.
func CanonicalEvaluationJSON(evaluation Evaluation) ([]byte, error) {
	if err := ValidateEvaluation(evaluation); err != nil {
		return nil, err
	}
	canonical := evaluation
	runtimeJSON, err := evidence.CanonicalRuntimeSignatureJSON(evaluation.TargetRuntime)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(runtimeJSON, &canonical.TargetRuntime); err != nil {
		return nil, fmt.Errorf("decode canonical target runtime: %w", err)
	}
	if evaluation.CompatibilityPolicy != nil {
		policy := *evaluation.CompatibilityPolicy
		policy.IgnoredDimensions = slices.Clone(evaluation.CompatibilityPolicy.IgnoredDimensions)
		slices.Sort(policy.IgnoredDimensions)
		canonical.CompatibilityPolicy = &policy
	}
	canonical.Regions = slices.Clone(evaluation.Regions)
	slices.SortFunc(canonical.Regions, func(a, b Region) int { return strings.Compare(a.Name, b.Name) })
	canonical.Rules = slices.Clone(evaluation.Rules)
	slices.SortFunc(canonical.Rules, func(a, b Rule) int { return strings.Compare(a.ID, b.ID) })
	canonical.Evidence = make([]EvidenceNode, len(evaluation.Evidence))
	for i, node := range evaluation.Evidence {
		canonical.Evidence[i] = node
		envelopeJSON, err := evidence.CanonicalJSON(node.Envelope)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(envelopeJSON, &canonical.Evidence[i].Envelope); err != nil {
			return nil, fmt.Errorf("decode canonical evidence envelope: %w", err)
		}
		canonical.Evidence[i].Uncertainties = slices.Clone(node.Uncertainties)
		slices.SortFunc(canonical.Evidence[i].Uncertainties, func(a, b MetricUncertainty) int {
			return strings.Compare(metricKey(a.Name, a.Semantics), metricKey(b.Name, b.Semantics))
		})
	}
	slices.SortFunc(canonical.Evidence, func(a, b EvidenceNode) int {
		aDigest, _ := evidence.Digest(a.Envelope)
		bDigest, _ := evidence.Digest(b.Envelope)
		return strings.Compare(aDigest, bDigest)
	})
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("encode canonical gate evaluation: %w", err)
	}
	return encoded, nil
}

func EvaluationDigest(evaluation Evaluation) (string, error) {
	canonical, err := CanonicalEvaluationJSON(evaluation)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
