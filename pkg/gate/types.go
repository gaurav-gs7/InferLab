// Package gate evaluates whether exact observed evidence is sufficient and
// safe for an inference change, without converting uncertainty into certainty.
package gate

import (
	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

const (
	EvaluationSchema     = "inferlab.gate-evaluation"
	ResultSchema         = "inferlab.gate-result"
	CurrentSchemaVersion = "1.0"
	MaxDocumentBytes     = 8 << 20
)

type Decision string

const (
	DecisionPass         Decision = "PASS"
	DecisionBlock        Decision = "BLOCK"
	DecisionInconclusive Decision = "INCONCLUSIVE"
)

type Direction string

const (
	DirectionAtMost  Direction = "at-most"
	DirectionAtLeast Direction = "at-least"
)

type FaultType string

const (
	FaultNone             FaultType = "none"
	FaultReplicaLoss      FaultType = "replica-loss"
	FaultLongContextSpike FaultType = "long-context-spike"
)

type UncertaintyMethod string

const (
	UncertaintyBootstrap UncertaintyMethod = "bootstrap-standard-error"
	UncertaintySample    UncertaintyMethod = "sample-standard-error"
	UncertaintyBinomial  UncertaintyMethod = "binomial"
)

type FindingOutcome string

const (
	FindingPass         FindingOutcome = "pass"
	FindingBlock        FindingOutcome = "block"
	FindingInconclusive FindingOutcome = "inconclusive"
)

type FindingCode string

const (
	CodeWithinPolicy        FindingCode = "within-policy"
	CodeViolationRisk       FindingCode = "violation-risk"
	CodeUncertaintyOverlap  FindingCode = "uncertainty-overlap"
	CodeMissingCoverage     FindingCode = "missing-coverage"
	CodeOutOfDistribution   FindingCode = "out-of-distribution"
	CodeMissingMetric       FindingCode = "missing-metric"
	CodeInvalidMetric       FindingCode = "invalid-metric"
	CodeMissingUncertainty  FindingCode = "missing-uncertainty"
	CodeUnderSampled        FindingCode = "under-sampled"
	CodeIncompleteEvidence  FindingCode = "incomplete-evidence"
	CodeNonObservedEvidence FindingCode = "non-observed-evidence"
	CodeStaleEvidence       FindingCode = "stale-evidence"
	CodeFutureEvidence      FindingCode = "future-evidence"
	CodeRuntimeIncompatible FindingCode = "runtime-incompatible"
	CodeRuntimeUnknown      FindingCode = "runtime-unknown"
)

type AdmissionStatus string

const (
	AdmissionAccepted AdmissionStatus = "accepted"
	AdmissionRejected AdmissionStatus = "rejected"
)

// Evaluation is deterministic: EvaluatedAt, freshness limits, confidence,
// risk tolerance, target identity, coverage regions, and rules are explicit.
type Evaluation struct {
	Schema                      string                        `json:"schema"`
	SchemaVersion               string                        `json:"schema_version"`
	Name                        string                        `json:"name"`
	ChangeDigest                string                        `json:"change_digest"`
	EvaluatedAt                 string                        `json:"evaluated_at"`
	MaxEvidenceAgeSeconds       uint64                        `json:"max_evidence_age_seconds"`
	ConfidenceLevel             float64                       `json:"confidence_level"`
	MaximumViolationProbability float64                       `json:"maximum_violation_probability"`
	TargetRuntime               evidence.RuntimeSignature     `json:"target_runtime"`
	CompatibilityPolicy         *evidence.CompatibilityPolicy `json:"compatibility_policy,omitempty"`
	Regions                     []Region                      `json:"regions"`
	Rules                       []Rule                        `json:"rules"`
	Evidence                    []EvidenceNode                `json:"evidence"`
}

type Region struct {
	Name           string     `json:"name"`
	WorkloadDigest string     `json:"workload_digest"`
	Minimum        LoadShape  `json:"minimum"`
	Maximum        LoadShape  `json:"maximum"`
	Fault          FaultPoint `json:"fault"`
}

type LoadShape struct {
	Concurrency  uint32  `json:"concurrency"`
	PromptTokens uint64  `json:"prompt_tokens"`
	OutputTokens uint64  `json:"output_tokens"`
	TenantCount  uint32  `json:"tenant_count"`
	ArrivalRate  float64 `json:"arrival_rate_per_second"`
}

type WorkloadPoint struct {
	LoadShape
	Fault FaultPoint `json:"fault"`
}

// FaultPoint is deliberately narrow and bounded. Lost replicas are exact;
// long-context pressure is represented by prompt size and request fraction.
type FaultPoint struct {
	Type                FaultType `json:"type"`
	LostReplicas        uint32    `json:"lost_replicas,omitempty"`
	DurationSeconds     uint32    `json:"duration_seconds,omitempty"`
	LongContextTokens   uint64    `json:"long_context_tokens,omitempty"`
	LongContextFraction float64   `json:"long_context_fraction,omitempty"`
}

type Rule struct {
	ID             string    `json:"id"`
	Region         string    `json:"region"`
	Metric         string    `json:"metric"`
	Semantics      string    `json:"semantics"`
	Unit           string    `json:"unit"`
	Direction      Direction `json:"direction"`
	Threshold      float64   `json:"threshold"`
	MinimumSamples uint64    `json:"minimum_samples"`
}

type EvidenceNode struct {
	Envelope      evidence.Envelope   `json:"envelope"`
	Workload      WorkloadPoint       `json:"workload"`
	Uncertainties []MetricUncertainty `json:"uncertainties"`
}

type MetricUncertainty struct {
	Name          string            `json:"name"`
	Semantics     string            `json:"semantics"`
	Method        UncertaintyMethod `json:"method"`
	StandardError *float64          `json:"standard_error,omitempty"`
	Successes     *uint64           `json:"successes,omitempty"`
	Trials        *uint64           `json:"trials,omitempty"`
}

type Result struct {
	Schema           string              `json:"schema"`
	SchemaVersion    string              `json:"schema_version"`
	Evaluation       string              `json:"evaluation"`
	EvaluationDigest string              `json:"evaluation_digest"`
	ChangeDigest     string              `json:"change_digest"`
	Decision         Decision            `json:"decision"`
	Findings         []Finding           `json:"findings"`
	Coverage         []CoverageResult    `json:"coverage"`
	Admissions       []EvidenceAdmission `json:"admissions"`
	Graph            []GraphNode         `json:"graph"`
}

type Finding struct {
	RuleID               string         `json:"rule_id"`
	Region               string         `json:"region"`
	Outcome              FindingOutcome `json:"outcome"`
	Code                 FindingCode    `json:"code"`
	EvidenceDigest       string         `json:"evidence_digest,omitempty"`
	Metric               string         `json:"metric"`
	Semantics            string         `json:"semantics"`
	ObservedValue        *float64       `json:"observed_value,omitempty"`
	LowerBound           *float64       `json:"lower_bound,omitempty"`
	UpperBound           *float64       `json:"upper_bound,omitempty"`
	Threshold            float64        `json:"threshold"`
	ViolationProbability *float64       `json:"violation_probability,omitempty"`
	Message              string         `json:"message"`
}

type CoverageResult struct {
	Region           string   `json:"region"`
	CandidateDigests []string `json:"candidate_digests"`
	AdmittedDigests  []string `json:"admitted_digests"`
	Covered          bool     `json:"covered"`
}

type EvidenceAdmission struct {
	EvidenceDigest string                       `json:"evidence_digest"`
	Status         AdmissionStatus              `json:"status"`
	Codes          []FindingCode                `json:"codes,omitempty"`
	Compatibility  evidence.CompatibilityResult `json:"compatibility"`
}

type GraphNode struct {
	ID        string            `json:"id"`
	Kind      evidence.NodeKind `json:"kind"`
	DependsOn []string          `json:"depends_on"`
}

func NewEvaluation() Evaluation {
	return Evaluation{Schema: EvaluationSchema, SchemaVersion: CurrentSchemaVersion}
}
