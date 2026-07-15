package adapter

import (
	"encoding/json"
	"strings"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

func testRuntime(origin evidence.Origin) evidence.RuntimeSignature {
	signature := evidence.NewRuntimeSignature(origin)
	signature.Model = evidence.ModelIdentity{
		ID:                       "qwen-qwen2.5-7b-instruct-awq",
		Revision:                 strings.Repeat("1", 40),
		TokenizerID:              "qwen-qwen2.5-7b-instruct-awq",
		TokenizerRevision:        strings.Repeat("2", 40),
		Quantization:             "awq",
		QuantizationConfigDigest: "sha256:" + strings.Repeat("3", 64),
	}
	signature.Engine = evidence.EngineIdentity{
		Name:           "vllm",
		Revision:       strings.Repeat("4", 40),
		ContainerImage: "ghcr.io/vllm-project/vllm-openai@sha256:" + strings.Repeat("5", 64),
	}
	signature.Platform = evidence.PlatformIdentity{
		CUDAVersion: "12.8", DriverVersion: "570.133.20", GPUSKU: "nvidia-l4", GPUCount: 1, Topology: "single-gpu",
	}
	signature.Scheduler = evidence.SchedulerIdentity{Name: "vllm-v1", ConfigDigest: "sha256:" + strings.Repeat("6", 64)}
	signature.Kernels = []evidence.KernelIdentity{{Name: "flash-attention", Version: "2.7.4", ConfigDigest: "sha256:" + strings.Repeat("7", 64)}}
	return signature
}

func testInput(name string) Input {
	implementation, ok := Builtin(name)
	if !ok {
		panic("unknown test adapter " + name)
	}
	capabilities := implementation.Capabilities()
	classification := capabilities.Classifications[0]
	origin := evidence.OriginObserved
	if classification == evidence.ClassPredicted {
		origin = evidence.OriginDeclared
	}
	raw := producerReport{Schema: capabilities.Producer.ReportSchema, RunID: "fixture-run", Metrics: make([]OriginalMetric, 0, len(capabilities.Metrics))}
	for i, capability := range capabilities.Metrics {
		value := float64(i + 1)
		switch capability.SourceUnit {
		case "percent":
			value = 97
		case "probability":
			value = 0.97
		case "usd-per-1k-tokens":
			value = 0.0025
		}
		raw.Metrics = append(raw.Metrics, OriginalMetric{
			Name: capability.SourceName, Definition: capability.SourceDefinition, Value: value, Unit: capability.SourceUnit, SampleCount: 1000,
		})
	}
	report, err := json.Marshal(raw)
	if err != nil {
		panic(err)
	}
	return Input{
		Name:           name + "-evidence",
		Producer:       capabilities.Producer,
		Classification: classification,
		Completeness:   evidence.CompletenessComplete,
		Runtime:        testRuntime(origin),
		WorkloadDigest: "sha256:" + strings.Repeat("9", 64),
		Attempt:        1,
		StartedAt:      "2026-07-15T08:00:00Z",
		FinishedAt:     "2026-07-15T08:05:00Z",
		Report:         report,
	}
}
