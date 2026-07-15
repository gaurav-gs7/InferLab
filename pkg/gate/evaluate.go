package gate

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

const confidenceZ95 = 1.959963984540054

type indexedEvidence struct {
	node          EvidenceNode
	digest        string
	runtimeDigest string
	metrics       map[string]evidence.Metric
	uncertainties map[string]MetricUncertainty
	admission     EvidenceAdmission
}

// Evaluate applies every mandatory rule independently. Any block wins; a
// missing or uncertain rule prevents PASS; only an entirely passing set passes.
func Evaluate(evaluation Evaluation) (Result, error) {
	if err := ValidateEvaluation(evaluation); err != nil {
		return Result{}, err
	}
	evaluatedAt, _ := time.Parse(time.RFC3339Nano, evaluation.EvaluatedAt)
	indexed := make([]indexedEvidence, 0, len(evaluation.Evidence))
	for _, node := range evaluation.Evidence {
		entry, err := indexEvidence(node, evaluation, evaluatedAt)
		if err != nil {
			return Result{}, err
		}
		indexed = append(indexed, entry)
	}
	slices.SortFunc(indexed, func(a, b indexedEvidence) int { return strings.Compare(a.digest, b.digest) })

	regions := make(map[string]Region, len(evaluation.Regions))
	for _, region := range evaluation.Regions {
		regions[region.Name] = region
	}
	evaluationDigest, err := EvaluationDigest(evaluation)
	if err != nil {
		return Result{}, err
	}
	result := Result{
		Schema:           ResultSchema,
		SchemaVersion:    CurrentSchemaVersion,
		Evaluation:       evaluation.Name,
		EvaluationDigest: evaluationDigest,
		ChangeDigest:     evaluation.ChangeDigest,
		Admissions:       make([]EvidenceAdmission, 0, len(indexed)),
	}
	for _, entry := range indexed {
		result.Admissions = append(result.Admissions, entry.admission)
	}
	result.Coverage = evaluateCoverage(evaluation.Regions, indexed)
	for _, rule := range evaluation.Rules {
		result.Findings = append(result.Findings, evaluateRule(rule, regions[rule.Region], indexed, evaluation.MaximumViolationProbability)...)
	}
	sortFindings(result.Findings)
	result.Decision = decisionFromFindings(result.Findings)
	graph, err := buildGraph(evaluation, evaluationDigest, indexed, result.Findings)
	if err != nil {
		return Result{}, err
	}
	result.Graph = graph
	if err := ValidateResult(result); err != nil {
		return Result{}, err
	}
	return result, nil
}

func indexEvidence(node EvidenceNode, evaluation Evaluation, evaluatedAt time.Time) (indexedEvidence, error) {
	digest, err := evidence.Digest(node.Envelope)
	if err != nil {
		return indexedEvidence{}, err
	}
	runtimeDigest, err := evidence.RuntimeSignatureDigest(node.Envelope.Runtime)
	if err != nil {
		return indexedEvidence{}, err
	}
	compatibility, err := evidence.CompareRuntimeSignatures(node.Envelope.Runtime, evaluation.TargetRuntime, evaluation.CompatibilityPolicy)
	if err != nil {
		return indexedEvidence{}, err
	}
	codes := make([]FindingCode, 0, 4)
	if node.Envelope.Classification != evidence.ClassObserved {
		codes = append(codes, CodeNonObservedEvidence)
	}
	if node.Envelope.Completeness != evidence.CompletenessComplete {
		codes = append(codes, CodeIncompleteEvidence)
	}
	switch compatibility.Status {
	case evidence.CompatibilityMismatch:
		codes = append(codes, CodeRuntimeIncompatible)
	case evidence.CompatibilityUnknown:
		codes = append(codes, CodeRuntimeUnknown)
	}
	if node.Envelope.FinishedAt != "" {
		finished, parseErr := time.Parse(time.RFC3339Nano, node.Envelope.FinishedAt)
		if parseErr != nil {
			return indexedEvidence{}, parseErr
		}
		if finished.After(evaluatedAt) {
			codes = append(codes, CodeFutureEvidence)
		} else if evaluatedAt.Sub(finished) > time.Duration(evaluation.MaxEvidenceAgeSeconds)*time.Second {
			codes = append(codes, CodeStaleEvidence)
		}
	}
	codes = sortedUniqueCodes(codes)
	status := AdmissionAccepted
	if len(codes) > 0 {
		status = AdmissionRejected
	}
	metrics := make(map[string]evidence.Metric, len(node.Envelope.Metrics))
	for _, metric := range node.Envelope.Metrics {
		metrics[metricKey(metric.Name, metric.Semantics)] = metric
	}
	uncertainties := make(map[string]MetricUncertainty, len(node.Uncertainties))
	for _, uncertainty := range node.Uncertainties {
		uncertainties[metricKey(uncertainty.Name, uncertainty.Semantics)] = uncertainty
	}
	return indexedEvidence{
		node: node, digest: digest, runtimeDigest: runtimeDigest, metrics: metrics, uncertainties: uncertainties,
		admission: EvidenceAdmission{EvidenceDigest: digest, Status: status, Codes: codes, Compatibility: compatibility},
	}, nil
}

func evaluateCoverage(regions []Region, indexed []indexedEvidence) []CoverageResult {
	coverage := make([]CoverageResult, 0, len(regions))
	for _, region := range regions {
		item := CoverageResult{Region: region.Name}
		for _, entry := range indexed {
			if entry.node.Envelope.WorkloadDigest != region.WorkloadDigest {
				continue
			}
			item.CandidateDigests = append(item.CandidateDigests, entry.digest)
			if pointInRegion(entry.node.Workload, region) && entry.admission.Status == AdmissionAccepted {
				item.AdmittedDigests = append(item.AdmittedDigests, entry.digest)
			}
		}
		slices.Sort(item.CandidateDigests)
		slices.Sort(item.AdmittedDigests)
		item.Covered = len(item.AdmittedDigests) > 0
		coverage = append(coverage, item)
	}
	slices.SortFunc(coverage, func(a, b CoverageResult) int { return strings.Compare(a.Region, b.Region) })
	return coverage
}

func evaluateRule(rule Rule, region Region, indexed []indexedEvidence, maxViolationProbability float64) []Finding {
	workloadMatches := make([]indexedEvidence, 0)
	regionMatches := make([]indexedEvidence, 0)
	accepted := make([]indexedEvidence, 0)
	for _, entry := range indexed {
		if entry.node.Envelope.WorkloadDigest != region.WorkloadDigest {
			continue
		}
		workloadMatches = append(workloadMatches, entry)
		if !pointInRegion(entry.node.Workload, region) {
			continue
		}
		regionMatches = append(regionMatches, entry)
		if entry.admission.Status == AdmissionAccepted {
			accepted = append(accepted, entry)
		}
	}
	base := Finding{RuleID: rule.ID, Region: rule.Region, Metric: rule.Metric, Semantics: rule.Semantics, Threshold: rule.Threshold}
	if len(workloadMatches) == 0 {
		base.Outcome, base.Code, base.Message = FindingInconclusive, CodeMissingCoverage, "no evidence has the required workload digest"
		return []Finding{base}
	}
	if len(regionMatches) == 0 {
		base.Outcome, base.Code, base.Message = FindingInconclusive, CodeOutOfDistribution, "evidence workload points fall outside the required region or fault point"
		return []Finding{base}
	}
	if len(accepted) == 0 {
		byCode := make(map[FindingCode]Finding)
		for _, entry := range regionMatches {
			for _, code := range entry.admission.Codes {
				finding := base
				finding.Outcome = FindingInconclusive
				finding.Code = code
				finding.EvidenceDigest = entry.digest
				finding.Message = admissionMessage(code)
				if existing, exists := byCode[code]; !exists || finding.EvidenceDigest < existing.EvidenceDigest {
					byCode[code] = finding
				}
			}
		}
		findings := make([]Finding, 0, len(byCode))
		for _, finding := range byCode {
			findings = append(findings, finding)
		}
		sortFindings(findings)
		return findings
	}

	findings := make([]Finding, 0, len(accepted))
	for _, entry := range accepted {
		finding := base
		finding.EvidenceDigest = entry.digest
		metric, exists := entry.metrics[metricKey(rule.Metric, rule.Semantics)]
		if !exists || metric.Unit != rule.Unit {
			finding.Outcome, finding.Code, finding.Message = FindingInconclusive, CodeMissingMetric, "the exact metric name, semantics, and unit are absent; incompatible meanings are not pooled"
			findings = append(findings, finding)
			continue
		}
		value := metric.Value
		finding.ObservedValue = &value
		if !metricValueValid(metric) {
			finding.Outcome, finding.Code, finding.Message = FindingInconclusive, CodeInvalidMetric, "metric value is outside the supported physical or probability domain"
			findings = append(findings, finding)
			continue
		}
		if metric.SampleCount < rule.MinimumSamples {
			finding.Outcome, finding.Code, finding.Message = FindingInconclusive, CodeUnderSampled, fmt.Sprintf("sample_count %d is below required minimum %d", metric.SampleCount, rule.MinimumSamples)
			findings = append(findings, finding)
			continue
		}
		uncertainty, exists := entry.uncertainties[metricKey(rule.Metric, rule.Semantics)]
		if !exists {
			finding.Outcome, finding.Code, finding.Message = FindingInconclusive, CodeMissingUncertainty, "metric has no declared sampling uncertainty"
			findings = append(findings, finding)
			continue
		}
		standardError := uncertaintyStandardError(metric, uncertainty)
		lower, upper := confidenceBounds(metric, standardError, uncertainty)
		probability := violationProbability(metric.Value, standardError, rule.Threshold, rule.Direction)
		finding.LowerBound = &lower
		finding.UpperBound = &upper
		finding.ViolationProbability = &probability
		switch {
		case probability > maxViolationProbability:
			finding.Outcome, finding.Code = FindingBlock, CodeViolationRisk
			finding.Message = fmt.Sprintf("estimated violation probability %.6g exceeds maximum %.6g", probability, maxViolationProbability)
		case conservativeBoundPasses(lower, upper, rule):
			finding.Outcome, finding.Code = FindingPass, CodeWithinPolicy
			finding.Message = "the conservative 95% confidence bound and violation probability satisfy policy"
		default:
			finding.Outcome, finding.Code = FindingInconclusive, CodeUncertaintyOverlap
			finding.Message = "the 95% confidence interval crosses the policy threshold"
		}
		findings = append(findings, finding)
	}
	return []Finding{mostConservativeFinding(findings)}
}

func mostConservativeFinding(findings []Finding) Finding {
	best := findings[0]
	for _, candidate := range findings[1:] {
		bestRank, candidateRank := findingRank(best), findingRank(candidate)
		if candidateRank > bestRank {
			best = candidate
			continue
		}
		if candidateRank < bestRank {
			continue
		}
		if candidate.ViolationProbability != nil && (best.ViolationProbability == nil || *candidate.ViolationProbability > *best.ViolationProbability) {
			best = candidate
			continue
		}
		if candidate.ViolationProbability != nil && best.ViolationProbability != nil && *candidate.ViolationProbability < *best.ViolationProbability {
			continue
		}
		if (candidate.ViolationProbability == nil) == (best.ViolationProbability == nil) {
			candidateKey := string(candidate.Code) + "\x00" + candidate.EvidenceDigest
			bestKey := string(best.Code) + "\x00" + best.EvidenceDigest
			if candidateKey < bestKey {
				best = candidate
			}
		}
	}
	return best
}

func findingRank(finding Finding) int {
	switch finding.Outcome {
	case FindingBlock:
		return 3
	case FindingInconclusive:
		return 2
	default:
		return 1
	}
}

func admissionMessage(code FindingCode) string {
	switch code {
	case CodeIncompleteEvidence:
		return "partial evidence cannot satisfy a mandatory release rule"
	case CodeNonObservedEvidence:
		return "predicted, derived, or asserted evidence cannot satisfy an observed release rule"
	case CodeStaleEvidence:
		return "evidence exceeds the declared freshness limit"
	case CodeFutureEvidence:
		return "evidence finished after the declared evaluation time"
	case CodeRuntimeIncompatible:
		return "evidence runtime is incompatible with the target runtime"
	case CodeRuntimeUnknown:
		return "evidence or target runtime identity is unknown"
	default:
		return "evidence was rejected"
	}
}

func metricValueValid(metric evidence.Metric) bool {
	if !finite(metric.Value) {
		return false
	}
	switch metric.Unit {
	case "ratio", "probability":
		return metric.Value >= 0 && metric.Value <= 1
	case "milliseconds", "seconds", "usd", "tokens", "tokens_per_second", "count":
		return metric.Value >= 0
	default:
		return false
	}
}

func uncertaintyStandardError(metric evidence.Metric, uncertainty MetricUncertainty) float64 {
	if uncertainty.Method == UncertaintyBinomial {
		return math.Sqrt(metric.Value * (1 - metric.Value) / float64(*uncertainty.Trials))
	}
	return *uncertainty.StandardError
}

func confidenceBounds(metric evidence.Metric, standardError float64, uncertainty MetricUncertainty) (float64, float64) {
	if uncertainty.Method == UncertaintyBinomial {
		n := float64(*uncertainty.Trials)
		zSquared := confidenceZ95 * confidenceZ95
		center := (metric.Value + zSquared/(2*n)) / (1 + zSquared/n)
		halfWidth := confidenceZ95 * math.Sqrt(metric.Value*(1-metric.Value)/n+zSquared/(4*n*n)) / (1 + zSquared/n)
		return math.Max(0, center-halfWidth), math.Min(1, center+halfWidth)
	}
	lower := metric.Value - confidenceZ95*standardError
	upper := metric.Value + confidenceZ95*standardError
	if metric.Unit == "ratio" || metric.Unit == "probability" {
		lower = math.Max(0, lower)
		upper = math.Min(1, upper)
	} else {
		lower = math.Max(0, lower)
	}
	return lower, upper
}

func violationProbability(value, standardError, threshold float64, direction Direction) float64 {
	if standardError == 0 {
		if (direction == DirectionAtMost && value > threshold) || (direction == DirectionAtLeast && value < threshold) {
			return 1
		}
		return 0
	}
	if direction == DirectionAtMost {
		return normalCDF((value - threshold) / standardError)
	}
	return normalCDF((threshold - value) / standardError)
}

func normalCDF(value float64) float64 { return 0.5 * math.Erfc(-value/math.Sqrt2) }

func conservativeBoundPasses(lower, upper float64, rule Rule) bool {
	if rule.Direction == DirectionAtMost {
		return upper <= rule.Threshold
	}
	return lower >= rule.Threshold
}

func decisionFromFindings(findings []Finding) Decision {
	hasInconclusive := false
	for _, finding := range findings {
		if finding.Outcome == FindingBlock {
			return DecisionBlock
		}
		if finding.Outcome == FindingInconclusive {
			hasInconclusive = true
		}
	}
	if hasInconclusive {
		return DecisionInconclusive
	}
	return DecisionPass
}

func sortFindings(findings []Finding) {
	slices.SortFunc(findings, func(a, b Finding) int {
		left := strings.Join([]string{a.RuleID, a.Region, string(a.Outcome), string(a.Code), a.EvidenceDigest}, "\x00")
		right := strings.Join([]string{b.RuleID, b.Region, string(b.Outcome), string(b.Code), b.EvidenceDigest}, "\x00")
		return strings.Compare(left, right)
	})
}
