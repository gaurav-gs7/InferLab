package gate

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

func TestValidateEvaluationBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Evaluation)
	}{
		{name: "schema", mutate: func(value *Evaluation) { value.Schema = "other" }},
		{name: "identity", mutate: func(value *Evaluation) { value.Name = "Bad Name" }},
		{name: "change digest", mutate: func(value *Evaluation) { value.ChangeDigest = "bad" }},
		{name: "evaluated at", mutate: func(value *Evaluation) { value.EvaluatedAt = "today" }},
		{name: "zero evidence age", mutate: func(value *Evaluation) { value.MaxEvidenceAgeSeconds = 0 }},
		{name: "excessive evidence age", mutate: func(value *Evaluation) { value.MaxEvidenceAgeSeconds = 90*24*60*60 + 1 }},
		{name: "confidence", mutate: func(value *Evaluation) { value.ConfidenceLevel = 0.90 }},
		{name: "violation probability nan", mutate: func(value *Evaluation) { value.MaximumViolationProbability = math.NaN() }},
		{name: "violation probability zero", mutate: func(value *Evaluation) { value.MaximumViolationProbability = 0 }},
		{name: "violation probability high", mutate: func(value *Evaluation) { value.MaximumViolationProbability = 0.51 }},
		{name: "target runtime", mutate: func(value *Evaluation) { value.TargetRuntime.Schema = "other" }},
		{name: "unknown target dimension", mutate: func(value *Evaluation) { value.TargetRuntime.Platform.DriverVersion = "" }},
		{name: "compatibility policy", mutate: func(value *Evaluation) {
			value.CompatibilityPolicy = &evidence.CompatibilityPolicy{Name: "policy", Version: "latest"}
		}},
		{name: "empty regions", mutate: func(value *Evaluation) { value.Regions = nil }},
		{name: "too many regions", mutate: func(value *Evaluation) { value.Regions = make([]Region, maxRegions+1) }},
		{name: "region identity", mutate: func(value *Evaluation) { value.Regions[0].Name = "Bad Name" }},
		{name: "region digest", mutate: func(value *Evaluation) { value.Regions[0].WorkloadDigest = "bad" }},
		{name: "duplicate region", mutate: func(value *Evaluation) { value.Regions = append(value.Regions, value.Regions[0]) }},
		{name: "zero minimum bound", mutate: func(value *Evaluation) { value.Regions[0].Minimum.Concurrency = 0 }},
		{name: "reversed bound", mutate: func(value *Evaluation) { value.Regions[0].Maximum.OutputTokens = 1 }},
		{name: "arrival bound", mutate: func(value *Evaluation) { value.Regions[0].Minimum.ArrivalRate = math.NaN() }},
		{name: "region fault", mutate: func(value *Evaluation) { value.Regions[0].Fault = FaultPoint{Type: FaultNone, LostReplicas: 1} }},
		{name: "empty rules", mutate: func(value *Evaluation) { value.Rules = nil }},
		{name: "too many rules", mutate: func(value *Evaluation) { value.Rules = make([]Rule, maxRules+1) }},
		{name: "rule identity", mutate: func(value *Evaluation) { value.Rules[0].ID = "Bad ID" }},
		{name: "duplicate rule", mutate: func(value *Evaluation) { value.Rules = append(value.Rules, value.Rules[0]) }},
		{name: "unknown region", mutate: func(value *Evaluation) { value.Rules[0].Region = "missing" }},
		{name: "unsupported metric", mutate: func(value *Evaluation) { value.Rules[0].Metric = "invented" }},
		{name: "changed metric contract", mutate: func(value *Evaluation) { value.Rules[0].Unit = "seconds" }},
		{name: "nonfinite threshold", mutate: func(value *Evaluation) { value.Rules[0].Threshold = math.Inf(1) }},
		{name: "zero minimum samples", mutate: func(value *Evaluation) { value.Rules[0].MinimumSamples = 0 }},
		{name: "ratio threshold", mutate: func(value *Evaluation) {
			for i := range value.Rules {
				if value.Rules[i].Unit == "ratio" {
					value.Rules[i].Threshold = 1.1
					return
				}
			}
		}},
		{name: "too much evidence", mutate: func(value *Evaluation) { value.Evidence = make([]EvidenceNode, maxEvidence+1) }},
		{name: "invalid evidence envelope", mutate: func(value *Evaluation) { value.Evidence[0].Envelope.Schema = "other" }},
		{name: "invalid workload", mutate: func(value *Evaluation) { value.Evidence[0].Workload.Concurrency = 0 }},
		{name: "invalid evidence fault", mutate: func(value *Evaluation) { value.Evidence[0].Workload.Fault = FaultPoint{Type: FaultReplicaLoss} }},
		{name: "duplicate evidence", mutate: func(value *Evaluation) { value.Evidence = append(value.Evidence, value.Evidence[0]) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			evaluation := validGateEvaluation()
			tt.mutate(&evaluation)
			if err := ValidateEvaluation(evaluation); !errors.Is(err, ErrInvalidEvaluation) {
				t.Fatalf("ValidateEvaluation() error = %v, want %v", err, ErrInvalidEvaluation)
			}
		})
	}
}

func TestValidateEvidenceUncertaintyBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*EvidenceNode)
	}{
		{name: "missing metric", mutate: func(value *EvidenceNode) { value.Uncertainties[0].Name = "missing" }},
		{name: "duplicate uncertainty", mutate: func(value *EvidenceNode) { value.Uncertainties[1] = value.Uncertainties[0] }},
		{name: "goodput bootstrap", mutate: func(value *EvidenceNode) {
			for i := range value.Uncertainties {
				if value.Uncertainties[i].Name == MetricRequestGoodput {
					value.Uncertainties[i] = MetricUncertainty{Name: MetricRequestGoodput, Semantics: metricCatalog[MetricRequestGoodput].semantics, Method: UncertaintyBootstrap, StandardError: floatPointer(0.1)}
				}
			}
		}},
		{name: "missing standard error", mutate: func(value *EvidenceNode) { value.Uncertainties[0].StandardError = nil }},
		{name: "negative standard error", mutate: func(value *EvidenceNode) { value.Uncertainties[0].StandardError = floatPointer(-1) }},
		{name: "bootstrap with trials", mutate: func(value *EvidenceNode) { value.Uncertainties[0].Trials = uintPointer(1) }},
		{name: "invalid binomial fields", mutate: func(value *EvidenceNode) {
			for i := range value.Uncertainties {
				if value.Uncertainties[i].Method == UncertaintyBinomial {
					value.Uncertainties[i].Trials = uintPointer(0)
				}
			}
		}},
		{name: "binomial requires probability unit", mutate: func(value *EvidenceNode) {
			for i := range value.Envelope.Metrics {
				if value.Envelope.Metrics[i].Name == MetricRequestGoodput {
					value.Envelope.Metrics[i].Unit = "count"
				}
			}
		}},
		{name: "binomial only for goodput", mutate: func(value *EvidenceNode) {
			value.Envelope.Metrics[0].Unit = "ratio"
			value.Envelope.Metrics[0].Value = 0.7
			value.Uncertainties[0] = MetricUncertainty{Name: value.Envelope.Metrics[0].Name, Semantics: value.Envelope.Metrics[0].Semantics, Method: UncertaintyBinomial, Successes: uintPointer(700), Trials: uintPointer(1000)}
		}},
		{name: "trials differ from samples", mutate: func(value *EvidenceNode) {
			for i := range value.Uncertainties {
				if value.Uncertainties[i].Method == UncertaintyBinomial {
					value.Uncertainties[i].Trials = uintPointer(999)
					value.Uncertainties[i].Successes = uintPointer(969)
				}
			}
		}},
		{name: "ratio differs from metric", mutate: func(value *EvidenceNode) {
			for i := range value.Uncertainties {
				if value.Uncertainties[i].Method == UncertaintyBinomial {
					value.Uncertainties[i].Successes = uintPointer(900)
				}
			}
		}},
		{name: "unsupported method", mutate: func(value *EvidenceNode) { value.Uncertainties[0].Method = UncertaintyMethod("bayesian") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := validGateEvaluation().Evidence[0]
			tt.mutate(&node)
			if err := validateEvidenceNode(node); err == nil {
				t.Fatal("validateEvidenceNode() accepted invalid uncertainty")
			}
		})
	}
}

func TestFaultPointContract(t *testing.T) {
	t.Parallel()

	valid := []FaultPoint{
		{Type: FaultNone},
		{Type: FaultReplicaLoss, LostReplicas: 1, DurationSeconds: 60},
		{Type: FaultLongContextSpike, LongContextTokens: 8192, LongContextFraction: 0.25},
	}
	for _, fault := range valid {
		if err := ValidateFaultPoint(fault); err != nil {
			t.Fatalf("ValidateFaultPoint(%+v) error = %v", fault, err)
		}
	}
	invalid := []FaultPoint{
		{Type: FaultNone, LongContextFraction: math.NaN()},
		{Type: FaultNone, DurationSeconds: 1},
		{Type: FaultReplicaLoss, LostReplicas: 2, DurationSeconds: 1},
		{Type: FaultReplicaLoss, LostReplicas: 1, DurationSeconds: 601},
		{Type: FaultLongContextSpike, LongContextTokens: 4095, LongContextFraction: 0.25},
		{Type: FaultLongContextSpike, LongContextTokens: 8192, LongContextFraction: 0.255},
		{Type: FaultType("network-partition")},
	}
	for _, fault := range invalid {
		if err := ValidateFaultPoint(fault); !errors.Is(err, ErrUnsupportedFault) {
			t.Fatalf("ValidateFaultPoint(%+v) error = %v", fault, err)
		}
	}
}

func TestResultPrimitiveValidators(t *testing.T) {
	t.Parallel()

	complete := Finding{
		RuleID: "rule", Region: "region", Metric: "metric", Semantics: "semantics", EvidenceDigest: "sha256:" + strings.Repeat("a", 64),
		ObservedValue: floatPointer(1), LowerBound: floatPointer(0.9), UpperBound: floatPointer(1.1), ViolationProbability: floatPointer(0.01), Threshold: 2, Message: "message",
	}
	passing := complete
	passing.Outcome, passing.Code = FindingPass, CodeWithinPolicy
	blocking := complete
	blocking.Outcome, blocking.Code = FindingBlock, CodeViolationRisk
	overlap := complete
	overlap.Outcome, overlap.Code = FindingInconclusive, CodeUncertaintyOverlap
	for _, finding := range []Finding{passing, blocking, overlap, {Outcome: FindingInconclusive, Code: CodeMissingCoverage}} {
		if err := validateFindingShape(finding); err != nil {
			t.Fatalf("validateFindingShape(%+v) error = %v", finding, err)
		}
	}
	invalidFindings := []Finding{
		{Outcome: FindingPass, Code: CodeViolationRisk},
		{Outcome: FindingBlock, Code: CodeWithinPolicy},
		{Outcome: FindingInconclusive, Code: CodeWithinPolicy},
		{Outcome: FindingInconclusive, Code: CodeUncertaintyOverlap},
		{Outcome: FindingOutcome("unknown"), Code: CodeMissingCoverage},
	}
	for _, finding := range invalidFindings {
		if err := validateFindingShape(finding); err == nil {
			t.Fatalf("validateFindingShape(%+v) accepted invalid shape", finding)
		}
	}

	digest := "sha256:" + strings.Repeat("a", 64)
	if got, err := validateDigestSet([]string{digest}); err != nil || len(got) != 1 {
		t.Fatalf("validateDigestSet() = %v, %v", got, err)
	}
	for _, values := range [][]string{{"bad"}, {digest, digest}} {
		if _, err := validateDigestSet(values); err == nil {
			t.Fatalf("validateDigestSet(%v) accepted invalid set", values)
		}
	}
	if !validMessage("bounded diagnostic") || validMessage("") || validMessage("line one\nline two") || validMessage(strings.Repeat("x", 1025)) {
		t.Fatal("validMessage() contract mismatch")
	}
	if !knownDimension(evidence.DimensionDriverVersion) || knownDimension(evidence.Dimension("invented")) {
		t.Fatal("knownDimension() contract mismatch")
	}
	if !validFindingCode(CodeWithinPolicy) || validFindingCode(FindingCode("invented")) {
		t.Fatal("validFindingCode() contract mismatch")
	}
	if !isAdmissionCode(CodeStaleEvidence) || isAdmissionCode(CodeWithinPolicy) {
		t.Fatal("isAdmissionCode() contract mismatch")
	}
	if got := sortedUniqueCodes([]FindingCode{CodeStaleEvidence, CodeFutureEvidence, CodeStaleEvidence}); !slices.Equal(got, []FindingCode{CodeFutureEvidence, CodeStaleEvidence}) {
		t.Fatalf("sortedUniqueCodes() = %v", got)
	}
}

func TestCompatibilityResultValidation(t *testing.T) {
	t.Parallel()

	valid := []evidence.CompatibilityResult{
		{Status: evidence.CompatibilityExact},
		{Status: evidence.CompatibilityByPolicy, PolicyName: "policy", PolicyVersion: "1.0", IgnoredDifferences: []evidence.Dimension{evidence.DimensionDriverVersion}},
		{Status: evidence.CompatibilityMismatch, Differences: []evidence.Dimension{evidence.DimensionDriverVersion}},
		{Status: evidence.CompatibilityUnknown, UnknownDimensions: []evidence.Dimension{evidence.DimensionDriverVersion}},
	}
	for _, result := range valid {
		if err := validateCompatibilityResult(result); err != nil {
			t.Fatalf("validateCompatibilityResult(%+v) error = %v", result, err)
		}
	}
	invalid := []evidence.CompatibilityResult{
		{},
		{Status: evidence.CompatibilityExact, PolicyName: "policy"},
		{Status: evidence.CompatibilityExact, Differences: []evidence.Dimension{evidence.DimensionDriverVersion}},
		{Status: evidence.CompatibilityExact, Differences: []evidence.Dimension{"invented"}},
		{Status: evidence.CompatibilityMismatch, Differences: []evidence.Dimension{evidence.DimensionDriverVersion, evidence.DimensionDriverVersion}},
		{Status: evidence.CompatibilityByPolicy, PolicyName: "policy", PolicyVersion: "1.0"},
		{Status: evidence.CompatibilityMismatch},
		{Status: evidence.CompatibilityUnknown},
		{Status: evidence.CompatibilityUnknown, Differences: []evidence.Dimension{evidence.DimensionDriverVersion}, UnknownDimensions: []evidence.Dimension{evidence.DimensionCUDAVersion}},
		{Status: evidence.CompatibilityMismatch, Differences: []evidence.Dimension{evidence.DimensionDriverVersion}, IgnoredDifferences: []evidence.Dimension{evidence.DimensionCUDAVersion}},
	}
	for _, result := range invalid {
		if err := validateCompatibilityResult(result); err == nil {
			t.Fatalf("validateCompatibilityResult(%+v) accepted invalid result", result)
		}
	}
}

func TestValidateResultBoundaries(t *testing.T) {
	t.Parallel()

	digest := "sha256:" + strings.Repeat("c", 64)
	tests := []struct {
		name   string
		mutate func(*Result)
	}{
		{name: "schema", mutate: func(value *Result) { value.Schema = "other" }},
		{name: "identity", mutate: func(value *Result) { value.Evaluation = "Bad Name" }},
		{name: "evaluation digest", mutate: func(value *Result) { value.EvaluationDigest = "bad" }},
		{name: "decision", mutate: func(value *Result) { value.Decision = Decision("MAYBE") }},
		{name: "empty findings", mutate: func(value *Result) { value.Findings = nil }},
		{name: "bounded collections", mutate: func(value *Result) { value.Findings = make([]Finding, maxRules*6+1) }},
		{name: "admission digest", mutate: func(value *Result) { value.Admissions[0].EvidenceDigest = "bad" }},
		{name: "duplicate admission", mutate: func(value *Result) { value.Admissions = append(value.Admissions, value.Admissions[0]) }},
		{name: "accepted admission codes", mutate: func(value *Result) { value.Admissions[0].Codes = []FindingCode{CodeStaleEvidence} }},
		{name: "invalid admission status", mutate: func(value *Result) { value.Admissions[0].Status = AdmissionStatus("unknown") }},
		{name: "invalid admission code", mutate: func(value *Result) {
			value.Admissions[0].Status = AdmissionRejected
			value.Admissions[0].Codes = []FindingCode{"invented"}
		}},
		{name: "duplicate admission code", mutate: func(value *Result) {
			value.Admissions[0].Status = AdmissionRejected
			value.Admissions[0].Codes = []FindingCode{CodeStaleEvidence, CodeStaleEvidence}
		}},
		{name: "invalid compatibility", mutate: func(value *Result) {
			value.Admissions[0].Compatibility.Status = evidence.CompatibilityStatus("invented")
		}},
		{name: "finding identity", mutate: func(value *Result) { value.Findings[0].RuleID = "Bad ID" }},
		{name: "finding outcome", mutate: func(value *Result) { value.Findings[0].Outcome = FindingOutcome("unknown") }},
		{name: "finding code", mutate: func(value *Result) { value.Findings[0].Code = FindingCode("invented") }},
		{name: "unknown finding evidence", mutate: func(value *Result) { value.Findings[0].EvidenceDigest = digest }},
		{name: "finding uses rejected evidence", mutate: func(value *Result) {
			value.Admissions[0].Status = AdmissionRejected
			value.Admissions[0].Codes = []FindingCode{CodeStaleEvidence}
		}},
		{name: "finding rejection mismatch", mutate: func(value *Result) {
			value.Admissions[0].Status = AdmissionRejected
			value.Admissions[0].Codes = []FindingCode{CodeStaleEvidence}
			value.Findings[0] = Finding{RuleID: value.Findings[0].RuleID, Region: value.Findings[0].Region, Metric: value.Findings[0].Metric, Semantics: value.Findings[0].Semantics, Outcome: FindingInconclusive, Code: CodeFutureEvidence, EvidenceDigest: value.Admissions[0].EvidenceDigest, Threshold: value.Findings[0].Threshold, Message: "message"}
		}},
		{name: "finding threshold", mutate: func(value *Result) { value.Findings[0].Threshold = math.NaN() }},
		{name: "finding message", mutate: func(value *Result) { value.Findings[0].Message = "line one\nline two" }},
		{name: "finding nonfinite estimate", mutate: func(value *Result) { value.Findings[0].ObservedValue = floatPointer(math.Inf(1)) }},
		{name: "finding probability", mutate: func(value *Result) { value.Findings[0].ViolationProbability = floatPointer(1.1) }},
		{name: "finding half bounds", mutate: func(value *Result) { value.Findings[0].LowerBound = nil }},
		{name: "finding reversed bounds", mutate: func(value *Result) {
			value.Findings[0].LowerBound = floatPointer(2)
			value.Findings[0].UpperBound = floatPointer(1)
		}},
		{name: "finding shape", mutate: func(value *Result) { value.Findings[0].ObservedValue = nil }},
		{name: "coverage bound", mutate: func(value *Result) { value.Coverage[0].CandidateDigests = make([]string, maxEvidence+1) }},
		{name: "coverage region", mutate: func(value *Result) { value.Coverage[0].Region = "Bad Region" }},
		{name: "duplicate coverage", mutate: func(value *Result) { value.Coverage = append(value.Coverage, value.Coverage[0]) }},
		{name: "invalid candidate digest", mutate: func(value *Result) { value.Coverage[0].CandidateDigests = []string{"bad"} }},
		{name: "candidate without admission", mutate: func(value *Result) {
			value.Coverage[0].CandidateDigests = append(value.Coverage[0].CandidateDigests, digest)
		}},
		{name: "admitted is not candidate", mutate: func(value *Result) { value.Coverage[0].CandidateDigests = nil }},
		{name: "duplicate admitted digest", mutate: func(value *Result) {
			value.Coverage[0].AdmittedDigests = append(value.Coverage[0].AdmittedDigests, value.Coverage[0].AdmittedDigests[0])
		}},
		{name: "covered flag", mutate: func(value *Result) { value.Coverage[0].Covered = false }},
		{name: "coverage admits rejection", mutate: func(value *Result) {
			makeFindingsIndependent(value)
			value.Admissions[0].Status = AdmissionRejected
			value.Admissions[0].Codes = []FindingCode{CodeStaleEvidence}
		}},
		{name: "duplicate graph node", mutate: func(value *Result) { value.Graph = append(value.Graph, value.Graph[0]) }},
		{name: "invalid graph node", mutate: func(value *Result) { value.Graph[0].ID = "Bad ID" }},
		{name: "missing decision node", mutate: func(value *Result) {
			value.Graph = slices.DeleteFunc(value.Graph, func(node GraphNode) bool { return node.Kind == evidence.NodeDecision })
		}},
		{name: "missing policy closure", mutate: func(value *Result) {
			value.Graph = slices.DeleteFunc(value.Graph, func(node GraphNode) bool { return node.Kind == evidence.NodePolicy })
		}},
		{name: "missing evidence closure", mutate: func(value *Result) {
			value.Graph = slices.DeleteFunc(value.Graph, func(node GraphNode) bool { return node.Kind == evidence.NodeEvidence })
		}},
		{name: "decision dependencies", mutate: func(value *Result) {
			for i := range value.Graph {
				if value.Graph[i].Kind == evidence.NodeDecision {
					value.Graph[i].DependsOn = nil
				}
			}
		}},
		{name: "decision differs from findings", mutate: func(value *Result) { value.Decision = DecisionBlock }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Evaluate(validGateEvaluation())
			if err != nil {
				t.Fatal(err)
			}
			tt.mutate(&result)
			if err := ValidateResult(result); !errors.Is(err, ErrInvalidResult) {
				t.Fatalf("ValidateResult() error = %v, want %v", err, ErrInvalidResult)
			}
		})
	}
}

func makeFindingsIndependent(result *Result) {
	for i := range result.Findings {
		result.Findings[i].Outcome = FindingInconclusive
		result.Findings[i].Code = CodeMissingCoverage
		result.Findings[i].EvidenceDigest = ""
		result.Findings[i].ObservedValue = nil
		result.Findings[i].LowerBound = nil
		result.Findings[i].UpperBound = nil
		result.Findings[i].ViolationProbability = nil
	}
}
