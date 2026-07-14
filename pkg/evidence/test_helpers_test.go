package evidence

import "strings"

func validRuntimeSignature(origin Origin) RuntimeSignature {
	signature := NewRuntimeSignature(origin)
	signature.Model = ModelIdentity{
		ID:                       "qwen-qwen2.5-7b-instruct-awq",
		Revision:                 strings.Repeat("1", 40),
		TokenizerID:              "qwen-qwen2.5-7b-instruct-awq",
		TokenizerRevision:        strings.Repeat("2", 40),
		Quantization:             "awq",
		QuantizationConfigDigest: "sha256:" + strings.Repeat("3", 64),
	}
	signature.Engine = EngineIdentity{
		Name:           "vllm",
		Revision:       strings.Repeat("4", 40),
		ContainerImage: "ghcr.io/vllm-project/vllm-openai@sha256:" + strings.Repeat("5", 64),
	}
	signature.Platform = PlatformIdentity{
		CUDAVersion:   "12.8",
		DriverVersion: "570.133.20",
		GPUSKU:        "nvidia-l4",
		GPUCount:      1,
		Topology:      "single-gpu",
	}
	signature.Scheduler = SchedulerIdentity{
		Name:         "vllm-v1",
		ConfigDigest: "sha256:" + strings.Repeat("6", 64),
	}
	signature.Kernels = []KernelIdentity{
		{Name: "flash-attention", Version: "2.7.4", ConfigDigest: "sha256:" + strings.Repeat("7", 64)},
	}
	return signature
}

func validEnvelope() Envelope {
	envelope := NewEnvelope()
	envelope.Name = "guidellm-observation"
	envelope.Classification = ClassObserved
	envelope.Completeness = CompletenessComplete
	envelope.Source = Source{
		Tool:            "guidellm",
		ToolVersion:     "0.6.0",
		ReportSchema:    "guidellm-benchmark-v1",
		Adapter:         "guidellm-json",
		AdapterRevision: strings.Repeat("8", 40),
	}
	envelope.Runtime = validRuntimeSignature(OriginObserved)
	envelope.WorkloadDigest = "sha256:" + strings.Repeat("9", 64)
	envelope.Attempt = 1
	envelope.StartedAt = "2026-07-14T12:00:00Z"
	envelope.FinishedAt = "2026-07-14T12:05:00Z"
	envelope.Metrics = []Metric{
		{Name: "ttft-p99", Semantics: "guidellm-request-ttft-v1", Value: 812.5, Unit: "milliseconds", SampleCount: 1000},
		{Name: "request-goodput", Semantics: "guidellm-slo-goodput-v1", Value: 0.97, Unit: "ratio", SampleCount: 1000},
	}
	envelope.Artifacts = []Artifact{
		{Name: "raw-report", MediaType: "application/json", Digest: "sha256:" + strings.Repeat("a", 64)},
	}
	return envelope
}
