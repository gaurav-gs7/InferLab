package change

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*Document)
		wantErr error
		want    string
	}{
		{name: "valid"},
		{name: "schema", mutate: func(d *Document) { d.Schema = "other" }, wantErr: ErrUnsupportedSchema},
		{name: "version", mutate: func(d *Document) { d.SchemaVersion = "2.0" }, wantErr: ErrUnsupportedVersion},
		{name: "name", mutate: func(d *Document) { d.Name = "Invalid_Name" }, wantErr: ErrInvalidDocument, want: "DNS label"},
		{name: "same configuration", mutate: func(d *Document) { d.Candidate = d.Baseline }, wantErr: ErrInvalidDocument, want: "must differ"},
		{name: "unsupported engine", mutate: func(d *Document) { d.Candidate.Engine.Name = "sglang" }, wantErr: ErrUnsupportedFeature, want: "vllm"},
		{name: "mutable image", mutate: func(d *Document) { d.Candidate.Engine.Image = "ghcr.io/vllm-project/vllm-openai:latest" }, wantErr: ErrInvalidDocument, want: "sha256"},
		{name: "mutable model revision", mutate: func(d *Document) { d.Candidate.Model.Revision = "main" }, wantErr: ErrInvalidDocument, want: "immutable"},
		{name: "empty model id", mutate: func(d *Document) { d.Candidate.Model.ID = "" }, wantErr: ErrInvalidDocument, want: "1..256 bytes"},
		{name: "control character model id", mutate: func(d *Document) { d.Candidate.Model.ID = "model\nforged" }, wantErr: ErrInvalidDocument, want: "control characters"},
		{name: "unsupported quantization", mutate: func(d *Document) { d.Candidate.Model.Quantization = "fp8" }, wantErr: ErrUnsupportedFeature, want: "quantization"},
		{name: "unsupported hardware", mutate: func(d *Document) { d.Candidate.Hardware.Accelerator = "nvidia-h100" }, wantErr: ErrUnsupportedFeature, want: "nvidia-l4"},
		{name: "too many replicas", mutate: func(d *Document) { d.Candidate.Hardware.Replicas = 3 }, wantErr: ErrUnsupportedFeature, want: "replicas"},
		{name: "zero batching tokens", mutate: func(d *Document) { d.Candidate.Scheduler.MaxNumBatchedTokens = 0 }, wantErr: ErrInvalidDocument, want: "max_num_batched_tokens"},
		{name: "zero max sequences", mutate: func(d *Document) { d.Candidate.Scheduler.MaxSequences = 0 }, wantErr: ErrInvalidDocument, want: "max_sequences"},
		{name: "trace credentials", mutate: func(d *Document) { d.Workload.Trace = "s3://user:secret@bucket/key" }, wantErr: ErrInvalidDocument, want: "credential-free"},
		{name: "trace query", mutate: func(d *Document) { d.Workload.Trace = "s3://bucket/key?token=secret" }, wantErr: ErrInvalidDocument, want: "credential-free"},
		{name: "s3 trace without key", mutate: func(d *Document) { d.Workload.Trace = "s3://bucket" }, wantErr: ErrInvalidDocument, want: "bucket and object key"},
		{name: "file trace with host", mutate: func(d *Document) { d.Workload.Trace = "file://host/tmp/trace" }, wantErr: ErrInvalidDocument, want: "absolute local path"},
		{name: "unsupported trace scheme", mutate: func(d *Document) { d.Workload.Trace = "https://example.com/trace" }, wantErr: ErrUnsupportedFeature, want: "scheme"},
		{name: "invalid replay speed", mutate: func(d *Document) { d.Workload.ReplaySpeed = math.Inf(1) }, wantErr: ErrInvalidDocument, want: "replay_speed"},
		{name: "excessive replay speed", mutate: func(d *Document) { d.Workload.ReplaySpeed = 101 }, wantErr: ErrInvalidDocument, want: "replay_speed"},
		{name: "empty tenants", mutate: func(d *Document) { d.Workload.Tenants = nil }, wantErr: ErrInvalidDocument, want: "1..128"},
		{name: "empty tenant name", mutate: func(d *Document) { d.Workload.Tenants[0].Name = "" }, wantErr: ErrInvalidDocument, want: "1..256 bytes"},
		{name: "zero tenant weight", mutate: func(d *Document) { d.Workload.Tenants[0].Weight = 0 }, wantErr: ErrInvalidDocument, want: "weight"},
		{name: "duplicate tenant", mutate: func(d *Document) { d.Workload.Tenants[1].Name = d.Workload.Tenants[0].Name }, wantErr: ErrInvalidDocument, want: "repeats"},
		{name: "zero ttft", mutate: func(d *Document) { d.Policies.TTFTP99Milliseconds = 0 }, wantErr: ErrInvalidDocument, want: "ttft"},
		{name: "zero tpot", mutate: func(d *Document) { d.Policies.TPOTP99Milliseconds = 0 }, wantErr: ErrInvalidDocument, want: "tpot"},
		{name: "invalid fairness", mutate: func(d *Document) { d.Policies.MinimumFairnessIndex = 1.1 }, wantErr: ErrInvalidDocument, want: "fairness"},
		{name: "invalid cost", mutate: func(d *Document) { d.Policies.MaximumCostPerMillionTokensUSD = math.NaN() }, wantErr: ErrInvalidDocument, want: "cost"},
		{name: "unsafe violation probability", mutate: func(d *Document) { d.Policies.MaximumViolationProbability = 0.500001 }, wantErr: ErrInvalidDocument, want: "(0, 0.5]"},
		{name: "duplicate fault", mutate: func(d *Document) { d.Faults[1] = d.Faults[0] }, wantErr: ErrInvalidDocument, want: "repeats"},
		{name: "too many faults", mutate: func(d *Document) { d.Faults = make([]Fault, maxFaults+1) }, wantErr: ErrInvalidDocument, want: "maximum"},
		{name: "unsupported fault", mutate: func(d *Document) { d.Faults[0].Type = "cuda-oom" }, wantErr: ErrUnsupportedFeature, want: "cuda-oom"},
		{name: "replica probability", mutate: func(d *Document) { d.Faults[0].Probability = 0 }, wantErr: ErrInvalidDocument, want: "probability"},
		{name: "empty replica durations", mutate: func(d *Document) { d.Faults[0].DurationSeconds = nil }, wantErr: ErrInvalidDocument, want: "1..64"},
		{name: "unordered durations", mutate: func(d *Document) { d.Faults[0].DurationSeconds = []uint32{30, 15} }, wantErr: ErrInvalidDocument, want: "strictly increasing"},
		{name: "replica prompt tokens", mutate: func(d *Document) { d.Faults[0].PromptTokens = []uint64{4096} }, wantErr: ErrInvalidDocument, want: "not valid"},
		{name: "fault field mismatch", mutate: func(d *Document) { d.Faults[1].Probability = 0.2 }, wantErr: ErrInvalidDocument, want: "not valid"},
		{name: "long context duration", mutate: func(d *Document) { d.Faults[1].DurationSeconds = []uint32{1} }, wantErr: ErrInvalidDocument, want: "not valid"},
		{name: "empty long context points", mutate: func(d *Document) { d.Faults[1].PromptTokens = nil }, wantErr: ErrInvalidDocument, want: "1..64"},
		{name: "unordered long context points", mutate: func(d *Document) { d.Faults[1].PromptTokens = []uint64{8192, 4096} }, wantErr: ErrInvalidDocument, want: "strictly increasing"},
		{name: "zero experiment cost", mutate: func(d *Document) { d.Budget.MaximumExperimentCostUSD = 0 }, wantErr: ErrInvalidDocument, want: "experiment_cost"},
		{name: "zero GPU minutes", mutate: func(d *Document) { d.Budget.MaximumGPUMinutes = 0 }, wantErr: ErrInvalidDocument, want: "gpu_minutes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			document := validDocument()
			if tt.mutate != nil {
				tt.mutate(&document)
			}
			err := Validate(document)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.wantErr)
			}
			if tt.want != "" && !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate() error %q does not contain %q", err, tt.want)
			}
		})
	}
}

func TestValidateAllowsAbsoluteFileTrace(t *testing.T) {
	t.Parallel()
	document := validDocument()
	document.Workload.Trace = "file:///tmp/inferlab/trace.jsonl"
	if err := Validate(document); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}
