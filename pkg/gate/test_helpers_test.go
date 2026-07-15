package gate

import (
	"strings"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

func gateRuntime(origin evidence.Origin) evidence.RuntimeSignature {
	signature := evidence.NewRuntimeSignature(origin)
	signature.Model = evidence.ModelIdentity{
		ID: "qwen-qwen2.5-7b-instruct-awq", Revision: strings.Repeat("1", 40),
		TokenizerID: "qwen-qwen2.5-7b-instruct-awq", TokenizerRevision: strings.Repeat("2", 40),
		Quantization: "awq", QuantizationConfigDigest: "sha256:" + strings.Repeat("3", 64),
	}
	signature.Engine = evidence.EngineIdentity{Name: "vllm", Revision: strings.Repeat("4", 40), ContainerImage: "ghcr.io/vllm-project/vllm-openai@sha256:" + strings.Repeat("5", 64)}
	signature.Platform = evidence.PlatformIdentity{CUDAVersion: "12.8", DriverVersion: "570.133.20", GPUSKU: "nvidia-l4", GPUCount: 1, Topology: "single-gpu"}
	signature.Scheduler = evidence.SchedulerIdentity{Name: "vllm-v1", ConfigDigest: "sha256:" + strings.Repeat("6", 64)}
	signature.Kernels = []evidence.KernelIdentity{{Name: "flash-attention", Version: "2.7.4", ConfigDigest: "sha256:" + strings.Repeat("7", 64)}}
	return signature
}

func floatPointer(value float64) *float64 { return &value }
func uintPointer(value uint64) *uint64    { return &value }

func validGateEvaluation() Evaluation {
	workloadDigest := "sha256:" + strings.Repeat("9", 64)
	envelope := evidence.NewEnvelope()
	envelope.Name = "complete-observation"
	envelope.Classification = evidence.ClassObserved
	envelope.Completeness = evidence.CompletenessComplete
	envelope.Source = evidence.Source{Tool: "gate-fixture", ToolVersion: "1.0.0", ReportSchema: "gate-fixture-v1", Adapter: "gate-fixture", AdapterRevision: strings.Repeat("8", 64)}
	envelope.Runtime = gateRuntime(evidence.OriginObserved)
	envelope.WorkloadDigest = workloadDigest
	envelope.Attempt = 1
	envelope.StartedAt = "2026-07-15T08:00:00Z"
	envelope.FinishedAt = "2026-07-15T08:05:00Z"
	envelope.Metrics = []evidence.Metric{
		{Name: MetricTTFTP99, Semantics: metricCatalog[MetricTTFTP99].semantics, Value: 700, Unit: "milliseconds", SampleCount: 1000},
		{Name: MetricTPOTP99, Semantics: metricCatalog[MetricTPOTP99].semantics, Value: 30, Unit: "milliseconds", SampleCount: 1000},
		{Name: MetricITLP99, Semantics: metricCatalog[MetricITLP99].semantics, Value: 32, Unit: "milliseconds", SampleCount: 1000},
		{Name: MetricRequestGoodput, Semantics: metricCatalog[MetricRequestGoodput].semantics, Value: 0.97, Unit: "ratio", SampleCount: 1000},
		{Name: MetricFairnessIndex, Semantics: metricCatalog[MetricFairnessIndex].semantics, Value: 0.96, Unit: "ratio", SampleCount: 1000},
		{Name: MetricNoisyNeighborImpact, Semantics: metricCatalog[MetricNoisyNeighborImpact].semantics, Value: 0.10, Unit: "ratio", SampleCount: 1000},
		{Name: MetricRecoverySeconds, Semantics: metricCatalog[MetricRecoverySeconds].semantics, Value: 20, Unit: "seconds", SampleCount: 1000},
		{Name: MetricCostPerMillionTokens, Semantics: metricCatalog[MetricCostPerMillionTokens].semantics, Value: 2.5, Unit: "usd", SampleCount: 1000},
	}
	envelope.Artifacts = []evidence.Artifact{{Name: "raw-report", MediaType: "application/json", Digest: "sha256:" + strings.Repeat("a", 64)}}
	uncertainties := []MetricUncertainty{
		{Name: MetricTTFTP99, Semantics: metricCatalog[MetricTTFTP99].semantics, Method: UncertaintyBootstrap, StandardError: floatPointer(10)},
		{Name: MetricTPOTP99, Semantics: metricCatalog[MetricTPOTP99].semantics, Method: UncertaintyBootstrap, StandardError: floatPointer(1)},
		{Name: MetricITLP99, Semantics: metricCatalog[MetricITLP99].semantics, Method: UncertaintyBootstrap, StandardError: floatPointer(1)},
		{Name: MetricRequestGoodput, Semantics: metricCatalog[MetricRequestGoodput].semantics, Method: UncertaintyBinomial, Successes: uintPointer(970), Trials: uintPointer(1000)},
		{Name: MetricFairnessIndex, Semantics: metricCatalog[MetricFairnessIndex].semantics, Method: UncertaintyBootstrap, StandardError: floatPointer(0.005)},
		{Name: MetricNoisyNeighborImpact, Semantics: metricCatalog[MetricNoisyNeighborImpact].semantics, Method: UncertaintyBootstrap, StandardError: floatPointer(0.01)},
		{Name: MetricRecoverySeconds, Semantics: metricCatalog[MetricRecoverySeconds].semantics, Method: UncertaintyBootstrap, StandardError: floatPointer(1)},
		{Name: MetricCostPerMillionTokens, Semantics: metricCatalog[MetricCostPerMillionTokens].semantics, Method: UncertaintyBootstrap, StandardError: floatPointer(0.05)},
	}
	rules := make([]Rule, 0, len(metricCatalog))
	thresholds := map[string]float64{
		MetricTTFTP99: 1000, MetricTPOTP99: 50, MetricITLP99: 50, MetricRequestGoodput: 0.90,
		MetricFairnessIndex: 0.90, MetricNoisyNeighborImpact: 0.20, MetricRecoverySeconds: 30, MetricCostPerMillionTokens: 3,
	}
	for _, name := range []string{MetricTTFTP99, MetricTPOTP99, MetricITLP99, MetricRequestGoodput, MetricFairnessIndex, MetricNoisyNeighborImpact, MetricRecoverySeconds, MetricCostPerMillionTokens} {
		contract := metricCatalog[name]
		rules = append(rules, Rule{ID: name, Region: "baseline", Metric: name, Semantics: contract.semantics, Unit: contract.unit, Direction: contract.direction, Threshold: thresholds[name], MinimumSamples: 500})
	}
	evaluation := NewEvaluation()
	evaluation.Name = "release-gate-fixture"
	evaluation.ChangeDigest = "sha256:" + strings.Repeat("b", 64)
	evaluation.EvaluatedAt = "2026-07-15T09:00:00Z"
	evaluation.MaxEvidenceAgeSeconds = 7200
	evaluation.ConfidenceLevel = 0.95
	evaluation.MaximumViolationProbability = 0.05
	evaluation.TargetRuntime = gateRuntime(evidence.OriginObserved)
	evaluation.Regions = []Region{{
		Name: "baseline", WorkloadDigest: workloadDigest,
		Minimum: LoadShape{Concurrency: 8, PromptTokens: 128, OutputTokens: 32, TenantCount: 2, ArrivalRate: 5},
		Maximum: LoadShape{Concurrency: 32, PromptTokens: 4096, OutputTokens: 512, TenantCount: 8, ArrivalRate: 30},
		Fault:   FaultPoint{Type: FaultNone},
	}}
	evaluation.Rules = rules
	evaluation.Evidence = []EvidenceNode{{
		Envelope:      envelope,
		Workload:      WorkloadPoint{LoadShape: LoadShape{Concurrency: 16, PromptTokens: 512, OutputTokens: 128, TenantCount: 4, ArrivalRate: 12}, Fault: FaultPoint{Type: FaultNone}},
		Uncertainties: uncertainties,
	}}
	return evaluation
}
