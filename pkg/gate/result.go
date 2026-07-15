package gate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

func ValidateResult(result Result) error {
	if result.Schema != ResultSchema || result.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("%w: unsupported schema or version", ErrInvalidResult)
	}
	if !namePattern.MatchString(result.Evaluation) || !digestPattern.MatchString(result.EvaluationDigest) || !digestPattern.MatchString(result.ChangeDigest) {
		return fmt.Errorf("%w: evaluation, evaluation_digest, or change_digest is invalid", ErrInvalidResult)
	}
	if result.Decision != DecisionPass && result.Decision != DecisionBlock && result.Decision != DecisionInconclusive {
		return fmt.Errorf("%w: decision is invalid", ErrInvalidResult)
	}
	if len(result.Findings) == 0 {
		return fmt.Errorf("%w: findings must not be empty", ErrInvalidResult)
	}
	if len(result.Findings) > maxRules*6 || len(result.Coverage) > maxRegions || len(result.Admissions) > maxEvidence || len(result.Graph) > 8192 {
		return fmt.Errorf("%w: result collections exceed v1 bounds", ErrInvalidResult)
	}
	admissions := make(map[string]EvidenceAdmission, len(result.Admissions))
	for i, admission := range result.Admissions {
		if !digestPattern.MatchString(admission.EvidenceDigest) {
			return fmt.Errorf("%w: admissions[%d] has invalid digest", ErrInvalidResult, i)
		}
		if _, exists := admissions[admission.EvidenceDigest]; exists {
			return fmt.Errorf("%w: duplicate admission digest", ErrInvalidResult)
		}
		admissions[admission.EvidenceDigest] = admission
		if admission.Status == AdmissionAccepted {
			if len(admission.Codes) != 0 {
				return fmt.Errorf("%w: accepted admission has rejection codes", ErrInvalidResult)
			}
		} else if admission.Status != AdmissionRejected || len(admission.Codes) == 0 {
			return fmt.Errorf("%w: rejected admission requires codes", ErrInvalidResult)
		}
		seenCodes := make(map[FindingCode]struct{}, len(admission.Codes))
		for _, code := range admission.Codes {
			if !validFindingCode(code) {
				return fmt.Errorf("%w: admissions[%d] has invalid code", ErrInvalidResult, i)
			}
			if _, exists := seenCodes[code]; exists {
				return fmt.Errorf("%w: admissions[%d] has duplicate code", ErrInvalidResult, i)
			}
			seenCodes[code] = struct{}{}
		}
		if err := validateCompatibilityResult(admission.Compatibility); err != nil {
			return fmt.Errorf("%w: admissions[%d] compatibility: %v", ErrInvalidResult, i, err)
		}
	}

	for i, finding := range result.Findings {
		if !namePattern.MatchString(finding.RuleID) || !namePattern.MatchString(finding.Region) || !namePattern.MatchString(finding.Metric) || !namePattern.MatchString(finding.Semantics) {
			return fmt.Errorf("%w: findings[%d] has invalid identity", ErrInvalidResult, i)
		}
		if finding.Outcome != FindingPass && finding.Outcome != FindingBlock && finding.Outcome != FindingInconclusive || !validFindingCode(finding.Code) {
			return fmt.Errorf("%w: findings[%d] has invalid outcome or code", ErrInvalidResult, i)
		}
		if finding.EvidenceDigest != "" {
			admission, exists := admissions[finding.EvidenceDigest]
			if !exists {
				return fmt.Errorf("%w: findings[%d] references an unknown evidence digest", ErrInvalidResult, i)
			}
			if finding.Outcome == FindingPass || finding.Outcome == FindingBlock || !isAdmissionCode(finding.Code) {
				if admission.Status != AdmissionAccepted {
					return fmt.Errorf("%w: findings[%d] uses rejected evidence", ErrInvalidResult, i)
				}
			} else if admission.Status != AdmissionRejected || !slices.Contains(admission.Codes, finding.Code) {
				return fmt.Errorf("%w: findings[%d] rejection code differs from admission", ErrInvalidResult, i)
			}
		}
		if !finite(finding.Threshold) || !validMessage(finding.Message) {
			return fmt.Errorf("%w: findings[%d] has invalid threshold or message", ErrInvalidResult, i)
		}
		for _, value := range []*float64{finding.ObservedValue, finding.LowerBound, finding.UpperBound, finding.ViolationProbability} {
			if value != nil && !finite(*value) {
				return fmt.Errorf("%w: findings[%d] contains a non-finite value", ErrInvalidResult, i)
			}
		}
		if finding.ViolationProbability != nil && (*finding.ViolationProbability < 0 || *finding.ViolationProbability > 1) {
			return fmt.Errorf("%w: findings[%d] violation probability is outside [0,1]", ErrInvalidResult, i)
		}
		if (finding.LowerBound == nil) != (finding.UpperBound == nil) {
			return fmt.Errorf("%w: findings[%d] must contain both confidence bounds", ErrInvalidResult, i)
		}
		if finding.LowerBound != nil && *finding.LowerBound > *finding.UpperBound {
			return fmt.Errorf("%w: findings[%d] has reversed confidence bounds", ErrInvalidResult, i)
		}
		if err := validateFindingShape(finding); err != nil {
			return fmt.Errorf("%w: findings[%d]: %v", ErrInvalidResult, i, err)
		}
	}

	seenRegions := make(map[string]struct{}, len(result.Coverage))
	for i, coverage := range result.Coverage {
		if len(coverage.CandidateDigests) > maxEvidence || len(coverage.AdmittedDigests) > maxEvidence {
			return fmt.Errorf("%w: coverage[%d] exceeds evidence bounds", ErrInvalidResult, i)
		}
		if !namePattern.MatchString(coverage.Region) {
			return fmt.Errorf("%w: coverage[%d] has invalid region", ErrInvalidResult, i)
		}
		if _, exists := seenRegions[coverage.Region]; exists {
			return fmt.Errorf("%w: duplicate coverage region", ErrInvalidResult)
		}
		seenRegions[coverage.Region] = struct{}{}
		candidates, err := validateDigestSet(coverage.CandidateDigests)
		if err != nil {
			return fmt.Errorf("%w: coverage[%d] candidates: %v", ErrInvalidResult, i, err)
		}
		for digest := range candidates {
			if _, exists := admissions[digest]; !exists {
				return fmt.Errorf("%w: coverage[%d] candidate has no admission", ErrInvalidResult, i)
			}
		}
		for _, digest := range coverage.AdmittedDigests {
			if _, exists := candidates[digest]; !exists {
				return fmt.Errorf("%w: coverage[%d] admitted digest is not a candidate", ErrInvalidResult, i)
			}
			if admissions[digest].Status != AdmissionAccepted {
				return fmt.Errorf("%w: coverage[%d] admits rejected evidence", ErrInvalidResult, i)
			}
		}
		if _, err := validateDigestSet(coverage.AdmittedDigests); err != nil {
			return fmt.Errorf("%w: coverage[%d] admitted: %v", ErrInvalidResult, i, err)
		}
		if coverage.Covered != (len(coverage.AdmittedDigests) > 0) {
			return fmt.Errorf("%w: coverage[%d] covered flag is inconsistent", ErrInvalidResult, i)
		}
	}

	graph := evidence.NewValidityGraph()
	graphNodes := make(map[string]GraphNode, len(result.Graph))
	for i, node := range result.Graph {
		if _, exists := graphNodes[node.ID]; exists {
			return fmt.Errorf("%w: duplicate graph node %q", ErrInvalidResult, node.ID)
		}
		graphNodes[node.ID] = node
		if err := graph.Add(evidence.Node{ID: node.ID, Kind: node.Kind, DependsOn: node.DependsOn}); err != nil {
			return fmt.Errorf("%w: graph[%d]: %v", ErrInvalidResult, i, err)
		}
	}
	claimID := graphID("claim", result.EvaluationDigest)
	claim, exists := graphNodes[claimID]
	if !exists || claim.Kind != evidence.NodeClaim {
		return fmt.Errorf("%w: graph is missing the canonical evaluation claim", ErrInvalidResult)
	}
	decisionNode, exists := graphNodes["decision-"+result.Evaluation]
	if !exists || decisionNode.Kind != evidence.NodeDecision {
		return fmt.Errorf("%w: graph is missing the decision node", ErrInvalidResult)
	}
	policyIDs := make(map[string]struct{})
	for _, finding := range result.Findings {
		policyID := "policy-" + finding.RuleID
		policy, exists := graphNodes[policyID]
		if !exists || policy.Kind != evidence.NodePolicy || !slices.Contains(policy.DependsOn, claimID) {
			return fmt.Errorf("%w: graph is missing policy closure for %q", ErrInvalidResult, finding.RuleID)
		}
		if finding.EvidenceDigest != "" {
			evidenceID := graphID("evidence", finding.EvidenceDigest)
			evidenceNode, exists := graphNodes[evidenceID]
			if !exists || evidenceNode.Kind != evidence.NodeEvidence || !slices.Contains(policy.DependsOn, evidenceID) {
				return fmt.Errorf("%w: graph is missing evidence closure for %q", ErrInvalidResult, finding.RuleID)
			}
		}
		policyIDs[policyID] = struct{}{}
	}
	wantPolicies := sortedKeys(policyIDs)
	gotPolicies := slices.Clone(decisionNode.DependsOn)
	slices.Sort(gotPolicies)
	if !slices.Equal(wantPolicies, gotPolicies) {
		return fmt.Errorf("%w: decision dependencies do not match policy findings", ErrInvalidResult)
	}
	if expected := decisionFromFindings(result.Findings); expected != result.Decision {
		return fmt.Errorf("%w: decision %s is inconsistent with findings; expected %s", ErrInvalidResult, result.Decision, expected)
	}
	return nil
}

func validateFindingShape(finding Finding) error {
	completeEstimate := finding.EvidenceDigest != "" && finding.ObservedValue != nil && finding.LowerBound != nil && finding.UpperBound != nil && finding.ViolationProbability != nil
	switch finding.Outcome {
	case FindingPass:
		if finding.Code != CodeWithinPolicy || !completeEstimate {
			return fmt.Errorf("passing finding requires within-policy and a complete estimate")
		}
	case FindingBlock:
		if finding.Code != CodeViolationRisk || !completeEstimate {
			return fmt.Errorf("blocking finding requires violation-risk and a complete estimate")
		}
	case FindingInconclusive:
		if finding.Code == CodeWithinPolicy || finding.Code == CodeViolationRisk {
			return fmt.Errorf("inconclusive finding has a conclusive code")
		}
		if finding.Code == CodeUncertaintyOverlap && !completeEstimate {
			return fmt.Errorf("uncertainty-overlap requires a complete estimate")
		}
	default:
		return fmt.Errorf("unknown outcome")
	}
	return nil
}

func isAdmissionCode(code FindingCode) bool {
	switch code {
	case CodeIncompleteEvidence, CodeNonObservedEvidence, CodeStaleEvidence, CodeFutureEvidence, CodeRuntimeIncompatible, CodeRuntimeUnknown:
		return true
	default:
		return false
	}
}

func validateCompatibilityResult(result evidence.CompatibilityResult) error {
	if !validCompatibilityStatus(result.Status) {
		return fmt.Errorf("invalid status")
	}
	if (result.PolicyName == "") != (result.PolicyVersion == "") {
		return fmt.Errorf("policy name and version must appear together")
	}
	for name, dimensions := range map[string][]evidence.Dimension{
		"differences": result.Differences, "ignored_differences": result.IgnoredDifferences, "unknown_dimensions": result.UnknownDimensions,
	} {
		seen := make(map[evidence.Dimension]struct{}, len(dimensions))
		for _, dimension := range dimensions {
			if !knownDimension(dimension) {
				return fmt.Errorf("%s contains unknown dimension %q", name, dimension)
			}
			if _, exists := seen[dimension]; exists {
				return fmt.Errorf("%s contains duplicate dimension %q", name, dimension)
			}
			seen[dimension] = struct{}{}
		}
	}
	switch result.Status {
	case evidence.CompatibilityExact:
		if len(result.Differences)+len(result.IgnoredDifferences)+len(result.UnknownDimensions) != 0 {
			return fmt.Errorf("exact compatibility contains differences")
		}
	case evidence.CompatibilityByPolicy:
		if result.PolicyName == "" || len(result.IgnoredDifferences) == 0 || len(result.Differences)+len(result.UnknownDimensions) != 0 {
			return fmt.Errorf("compatible-by-policy is inconsistent")
		}
	case evidence.CompatibilityMismatch:
		if len(result.Differences) == 0 {
			return fmt.Errorf("incompatible result has no differences")
		}
	case evidence.CompatibilityUnknown:
		if len(result.UnknownDimensions) == 0 || len(result.Differences) != 0 {
			return fmt.Errorf("unknown result is inconsistent")
		}
	}
	if len(result.IgnoredDifferences) > 0 && result.PolicyName == "" {
		return fmt.Errorf("ignored differences require a policy")
	}
	return nil
}

func knownDimension(dimension evidence.Dimension) bool {
	switch dimension {
	case evidence.DimensionModelID, evidence.DimensionModelRevision, evidence.DimensionTokenizerID, evidence.DimensionTokenizerRevision,
		evidence.DimensionQuantization, evidence.DimensionQuantizationConfig, evidence.DimensionEngineName, evidence.DimensionEngineRevision,
		evidence.DimensionContainerImage, evidence.DimensionCUDAVersion, evidence.DimensionDriverVersion, evidence.DimensionGPUSKU,
		evidence.DimensionGPUCount, evidence.DimensionTopology, evidence.DimensionSchedulerName, evidence.DimensionSchedulerConfig, evidence.DimensionKernels:
		return true
	default:
		return false
	}
}

func validFindingCode(code FindingCode) bool {
	switch code {
	case CodeWithinPolicy, CodeViolationRisk, CodeUncertaintyOverlap, CodeMissingCoverage, CodeOutOfDistribution,
		CodeMissingMetric, CodeInvalidMetric, CodeMissingUncertainty, CodeUnderSampled, CodeIncompleteEvidence,
		CodeNonObservedEvidence, CodeStaleEvidence, CodeFutureEvidence, CodeRuntimeIncompatible, CodeRuntimeUnknown:
		return true
	default:
		return false
	}
}

func validCompatibilityStatus(status evidence.CompatibilityStatus) bool {
	return status == evidence.CompatibilityExact || status == evidence.CompatibilityByPolicy || status == evidence.CompatibilityMismatch || status == evidence.CompatibilityUnknown
}

func validMessage(message string) bool {
	if message == "" || len(message) > 1024 || !utf8.ValidString(message) {
		return false
	}
	for _, character := range message {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validateDigestSet(digests []string) (map[string]struct{}, error) {
	seen := make(map[string]struct{}, len(digests))
	for _, digest := range digests {
		if !digestPattern.MatchString(digest) {
			return nil, fmt.Errorf("invalid digest %q", digest)
		}
		if _, exists := seen[digest]; exists {
			return nil, fmt.Errorf("duplicate digest %q", digest)
		}
		seen[digest] = struct{}{}
	}
	return seen, nil
}

func CanonicalResultJSON(result Result) ([]byte, error) {
	if err := ValidateResult(result); err != nil {
		return nil, err
	}
	canonical := result
	canonical.Findings = slices.Clone(result.Findings)
	sortFindings(canonical.Findings)
	canonical.Coverage = slices.Clone(result.Coverage)
	for i := range canonical.Coverage {
		canonical.Coverage[i].CandidateDigests = slices.Clone(canonical.Coverage[i].CandidateDigests)
		canonical.Coverage[i].AdmittedDigests = slices.Clone(canonical.Coverage[i].AdmittedDigests)
		slices.Sort(canonical.Coverage[i].CandidateDigests)
		slices.Sort(canonical.Coverage[i].AdmittedDigests)
	}
	slices.SortFunc(canonical.Coverage, func(a, b CoverageResult) int { return strings.Compare(a.Region, b.Region) })
	canonical.Admissions = slices.Clone(result.Admissions)
	for i := range canonical.Admissions {
		canonical.Admissions[i].Codes = sortedUniqueCodes(canonical.Admissions[i].Codes)
		canonical.Admissions[i].Compatibility.Differences = sortedDimensions(canonical.Admissions[i].Compatibility.Differences)
		canonical.Admissions[i].Compatibility.IgnoredDifferences = sortedDimensions(canonical.Admissions[i].Compatibility.IgnoredDifferences)
		canonical.Admissions[i].Compatibility.UnknownDimensions = sortedDimensions(canonical.Admissions[i].Compatibility.UnknownDimensions)
	}
	slices.SortFunc(canonical.Admissions, func(a, b EvidenceAdmission) int { return strings.Compare(a.EvidenceDigest, b.EvidenceDigest) })
	canonical.Graph = slices.Clone(result.Graph)
	for i := range canonical.Graph {
		canonical.Graph[i].DependsOn = slices.Clone(canonical.Graph[i].DependsOn)
		slices.Sort(canonical.Graph[i].DependsOn)
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("encode canonical gate result: %w", err)
	}
	return encoded, nil
}

func ResultDigest(result Result) (string, error) {
	canonical, err := CanonicalResultJSON(result)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func sortedDimensions(dimensions []evidence.Dimension) []evidence.Dimension {
	result := slices.Clone(dimensions)
	slices.Sort(result)
	return result
}
