package change

import "strings"

func validDocument() Document {
	document := NewDocument()
	document.Name = "qwen-vllm-batching-change"
	document.Baseline = validConfiguration("a", 4096, 64)
	document.Candidate = validConfiguration("b", 8192, 128)
	document.Workload = Workload{
		Trace:       "s3://inferlab-test-traces/chat-v1.jsonl",
		ReplaySpeed: 4,
		Tenants: []Tenant{
			{Name: "interactive", Weight: 10},
			{Name: "batch", Weight: 1},
		},
	}
	document.Policies = Policies{
		TTFTP99Milliseconds:            1500,
		TPOTP99Milliseconds:            90,
		MinimumFairnessIndex:           0.95,
		MaximumCostPerMillionTokensUSD: 0.8,
		MaximumViolationProbability:    0.1,
	}
	document.Faults = []Fault{
		{Type: "replica-loss", Probability: 0.05, DurationSeconds: []uint32{15, 30, 60}},
		{Type: "long-context-spike", PromptTokens: []uint64{4096, 8192, 16384}},
	}
	document.Budget = Budget{MaximumExperimentCostUSD: 6, MaximumGPUMinutes: 45}
	return document
}

func validConfiguration(digestCharacter string, maxBatchedTokens uint64, maxSequences uint32) Configuration {
	return Configuration{
		Engine: Engine{
			Name:  "vllm",
			Image: "ghcr.io/vllm-project/vllm-openai@sha256:" + strings.Repeat(digestCharacter, 64),
		},
		Model: Model{
			ID:           "Qwen/Qwen2.5-7B-Instruct-AWQ",
			Revision:     "0123456789abcdef0123456789abcdef01234567",
			Quantization: "awq",
		},
		Hardware:  Hardware{Accelerator: "nvidia-l4", InstanceType: "g6.xlarge", Replicas: 1},
		Scheduler: Scheduler{MaxNumBatchedTokens: maxBatchedTokens, MaxSequences: maxSequences},
	}
}
