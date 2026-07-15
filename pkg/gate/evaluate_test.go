package gate

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

func TestEvaluatePassesAllSupportedPolicyFamilies(t *testing.T) {
	t.Parallel()
	evaluation := validGateEvaluation()
	result, err := Evaluate(evaluation)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != DecisionPass {
		t.Fatalf("decision = %s, want PASS; findings=%+v", result.Decision, result.Findings)
	}
	if len(result.Findings) != len(evaluation.Rules) {
		t.Fatalf("findings = %d, want %d", len(result.Findings), len(evaluation.Rules))
	}
	for _, finding := range result.Findings {
		if finding.Outcome != FindingPass || finding.Code != CodeWithinPolicy || finding.EvidenceDigest == "" || finding.ViolationProbability == nil {
			t.Fatalf("unexpected passing finding: %+v", finding)
		}
	}
	if len(result.Graph) == 0 || result.Graph[len(result.Graph)-1].Kind != evidence.NodeDecision {
		t.Fatal("result does not contain a closed evidence-to-decision graph")
	}
}

func TestMandatoryFailureCannotBeHiddenByAggregateGains(t *testing.T) {
	t.Parallel()
	evaluation := validGateEvaluation()
	for i := range evaluation.Evidence[0].Envelope.Metrics {
		metric := &evaluation.Evidence[0].Envelope.Metrics[i]
		switch metric.Name {
		case MetricTTFTP99:
			metric.Value = 1200
		case MetricCostPerMillionTokens:
			metric.Value = 0.01
		}
	}
	result, err := Evaluate(evaluation)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != DecisionBlock {
		t.Fatalf("decision = %s, want BLOCK", result.Decision)
	}
	foundBlock := false
	for _, finding := range result.Findings {
		if finding.RuleID == MetricTTFTP99 && finding.Outcome == FindingBlock {
			foundBlock = true
		}
	}
	if !foundBlock {
		t.Fatal("TTFT policy violation was hidden")
	}
}

func TestFailClosedEvidenceAdmissionAndCoverage(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		mutate   func(*Evaluation)
		wantCode FindingCode
	}{
		{name: "stale", mutate: func(e *Evaluation) {
			e.Evidence[0].Envelope.StartedAt = "2026-07-14T08:00:00Z"
			e.Evidence[0].Envelope.FinishedAt = "2026-07-14T08:05:00Z"
		}, wantCode: CodeStaleEvidence},
		{name: "future", mutate: func(e *Evaluation) {
			e.Evidence[0].Envelope.StartedAt = "2026-07-15T10:00:00Z"
			e.Evidence[0].Envelope.FinishedAt = "2026-07-15T10:05:00Z"
		}, wantCode: CodeFutureEvidence},
		{name: "incomplete", mutate: func(e *Evaluation) { e.Evidence[0].Envelope.Completeness = evidence.CompletenessPartial }, wantCode: CodeIncompleteEvidence},
		{name: "predicted", mutate: func(e *Evaluation) {
			e.Evidence[0].Envelope.Classification = evidence.ClassPredicted
			e.Evidence[0].Envelope.Runtime.Origin = evidence.OriginDeclared
		}, wantCode: CodeNonObservedEvidence},
		{name: "incompatible runtime", mutate: func(e *Evaluation) { e.Evidence[0].Envelope.Runtime.Platform.DriverVersion = "571.0" }, wantCode: CodeRuntimeIncompatible},
		{name: "unknown runtime", mutate: func(e *Evaluation) {
			e.Evidence[0].Envelope.Completeness = evidence.CompletenessPartial
			e.Evidence[0].Envelope.Runtime.Platform.DriverVersion = ""
		}, wantCode: CodeRuntimeUnknown},
		{name: "out of distribution", mutate: func(e *Evaluation) { e.Evidence[0].Workload.Concurrency = 128 }, wantCode: CodeOutOfDistribution},
		{name: "missing workload", mutate: func(e *Evaluation) { e.Regions[0].WorkloadDigest = "sha256:" + strings.Repeat("c", 64) }, wantCode: CodeMissingCoverage},
		{name: "under sampled", mutate: func(e *Evaluation) { e.Rules[0].MinimumSamples = 2000 }, wantCode: CodeUnderSampled},
		{name: "missing uncertainty", mutate: func(e *Evaluation) { e.Evidence[0].Uncertainties = e.Evidence[0].Uncertainties[1:] }, wantCode: CodeMissingUncertainty},
		{name: "incompatible semantics", mutate: func(e *Evaluation) {
			e.Evidence[0].Envelope.Metrics[0].Semantics = "server-handler-latency-v1"
			e.Evidence[0].Uncertainties[0].Semantics = "server-handler-latency-v1"
		}, wantCode: CodeMissingMetric},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			evaluation := validGateEvaluation()
			tt.mutate(&evaluation)
			result, err := Evaluate(evaluation)
			if err != nil {
				t.Fatal(err)
			}
			if result.Decision == DecisionPass {
				t.Fatalf("unsafe evidence produced PASS: %+v", result)
			}
			found := false
			for _, finding := range result.Findings {
				if finding.Code == tt.wantCode {
					found = true
				}
			}
			if !found {
				t.Fatalf("findings do not contain %s: %+v", tt.wantCode, result.Findings)
			}
		})
	}
}

func TestExplicitCompatibilityPolicyCanAdmitKnownDrift(t *testing.T) {
	t.Parallel()
	evaluation := validGateEvaluation()
	evaluation.Evidence[0].Envelope.Runtime.Platform.DriverVersion = "571.0"
	evaluation.CompatibilityPolicy = &evidence.CompatibilityPolicy{Name: "driver-patch-equivalence", Version: "1.0", IgnoredDimensions: []evidence.Dimension{evidence.DimensionDriverVersion}}
	result, err := Evaluate(evaluation)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != DecisionPass || result.Admissions[0].Compatibility.Status != evidence.CompatibilityByPolicy {
		t.Fatalf("result = %+v, want compatible-by-policy PASS", result)
	}
}

func TestUncertaintyOverlapIsInconclusive(t *testing.T) {
	t.Parallel()
	evaluation := validGateEvaluation()
	// Risk is below 5%, but the stricter two-sided 95% upper bound crosses 1000.
	evaluation.Evidence[0].Envelope.Metrics[0].Value = 982
	evaluation.Evidence[0].Uncertainties[0].StandardError = floatPointer(10)
	result, err := Evaluate(evaluation)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != DecisionInconclusive {
		t.Fatalf("decision = %s, want INCONCLUSIVE", result.Decision)
	}
	if result.Findings[len(result.Findings)-1].Code == "" {
		t.Fatal("missing stable finding code")
	}
	found := false
	for _, finding := range result.Findings {
		if finding.RuleID == MetricTTFTP99 && finding.Code == CodeUncertaintyOverlap {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing uncertainty overlap finding: %+v", result.Findings)
	}
}

func TestResultIsDeterministicAcrossPolicyOrder(t *testing.T) {
	t.Parallel()
	firstEvaluation := validGateEvaluation()
	secondEvaluation := validGateEvaluation()
	slices.Reverse(secondEvaluation.Rules)
	first, err := Evaluate(firstEvaluation)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Evaluate(secondEvaluation)
	if err != nil {
		t.Fatal(err)
	}
	firstDigest, err := ResultDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := ResultDigest(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("result digest depends on policy order: %s != %s", firstDigest, secondDigest)
	}
}

func TestEvaluationDigestBindsWorkloadAndUncertainty(t *testing.T) {
	t.Parallel()
	base := validGateEvaluation()
	baseDigest, err := EvaluationDigest(base)
	if err != nil {
		t.Fatal(err)
	}
	workloadChanged := validGateEvaluation()
	workloadChanged.Evidence[0].Workload.Concurrency++
	workloadDigest, err := EvaluationDigest(workloadChanged)
	if err != nil {
		t.Fatal(err)
	}
	uncertaintyChanged := validGateEvaluation()
	uncertaintyChanged.Evidence[0].Uncertainties[0].StandardError = floatPointer(11)
	uncertaintyDigest, err := EvaluationDigest(uncertaintyChanged)
	if err != nil {
		t.Fatal(err)
	}
	if baseDigest == workloadDigest || baseDigest == uncertaintyDigest || workloadDigest == uncertaintyDigest {
		t.Fatalf("evaluation provenance was not bound: %s %s %s", baseDigest, workloadDigest, uncertaintyDigest)
	}
}

func TestEvaluateBoundsFindingsAcrossManyAttempts(t *testing.T) {
	t.Parallel()
	evaluation := validGateEvaluation()
	prototype := evaluation.Evidence[0]
	evaluation.Evidence = make([]EvidenceNode, 0, 128)
	for attempt := 1; attempt <= 128; attempt++ {
		node := prototype
		node.Envelope.Metrics = slices.Clone(prototype.Envelope.Metrics)
		node.Envelope.Artifacts = slices.Clone(prototype.Envelope.Artifacts)
		node.Uncertainties = slices.Clone(prototype.Uncertainties)
		node.Envelope.Attempt = uint32(attempt)
		node.Envelope.Artifacts[0].Digest = "sha256:" + fmt.Sprintf("%064x", attempt)
		evaluation.Evidence = append(evaluation.Evidence, node)
	}
	result, err := Evaluate(evaluation)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision != DecisionPass || len(result.Findings) != len(evaluation.Rules) || len(result.Admissions) != 128 || len(result.Coverage[0].AdmittedDigests) != 128 {
		t.Fatalf("result was not bounded without losing admission coverage: findings=%d admissions=%d coverage=%d decision=%s", len(result.Findings), len(result.Admissions), len(result.Coverage[0].AdmittedDigests), result.Decision)
	}
}

func TestValidateEvaluationRejectsUnsupportedContractsAndFaults(t *testing.T) {
	t.Parallel()
	evaluation := validGateEvaluation()
	evaluation.Rules[0].Semantics = "similar-looking-ttft-v1"
	if err := ValidateEvaluation(evaluation); !errors.Is(err, ErrInvalidEvaluation) {
		t.Fatalf("error = %v, want invalid evaluation", err)
	}
	evaluation = validGateEvaluation()
	evaluation.Regions[0].Fault = FaultPoint{Type: "network-partition"}
	if err := ValidateEvaluation(evaluation); !errors.Is(err, ErrUnsupportedFault) {
		t.Fatalf("error = %v, want unsupported fault", err)
	}
}

func TestValidateEvaluationRejectsInventedBinomialSemantics(t *testing.T) {
	t.Parallel()
	evaluation := validGateEvaluation()
	for i := range evaluation.Evidence[0].Uncertainties {
		uncertainty := &evaluation.Evidence[0].Uncertainties[i]
		if uncertainty.Name == MetricFairnessIndex {
			uncertainty.Method = UncertaintyBinomial
			uncertainty.StandardError = nil
			uncertainty.Successes = uintPointer(960)
			uncertainty.Trials = uintPointer(1000)
		}
	}
	if err := ValidateEvaluation(evaluation); !errors.Is(err, ErrInvalidEvaluation) {
		t.Fatalf("error = %v, want invalid evaluation", err)
	}
}
