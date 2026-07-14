// Package change defines InferLab's versioned contract for evaluating an
// inference configuration change before production deployment.
package change

const (
	// Schema identifies InferLab inference-change documents.
	Schema = "inferlab.change"
	// CurrentSchemaVersion is the schema version emitted by this package.
	CurrentSchemaVersion = "1.0"
	// MaxDocumentBytes bounds untrusted inference-change documents.
	MaxDocumentBytes = 1 << 20
)

// Document compares a candidate inference configuration with a baseline under
// one workload, policy set, fault campaign, and experiment budget.
type Document struct {
	Schema        string        `json:"schema"`
	SchemaVersion string        `json:"schema_version"`
	Name          string        `json:"name"`
	Baseline      Configuration `json:"baseline"`
	Candidate     Configuration `json:"candidate"`
	Workload      Workload      `json:"workload"`
	Policies      Policies      `json:"policies"`
	Faults        []Fault       `json:"faults,omitempty"`
	Budget        Budget        `json:"budget"`
}

// Configuration is one immutable serving configuration evaluated by InferLab.
type Configuration struct {
	Engine    Engine    `json:"engine"`
	Model     Model     `json:"model"`
	Hardware  Hardware  `json:"hardware"`
	Scheduler Scheduler `json:"scheduler"`
}

// Engine identifies a serving engine and immutable container image.
type Engine struct {
	Name  string `json:"name"`
	Image string `json:"image"`
}

// Model identifies immutable model content and its quantization mode.
type Model struct {
	ID           string `json:"id"`
	Revision     string `json:"revision"`
	Quantization string `json:"quantization"`
}

// Hardware describes the accelerator shape used for a configuration.
type Hardware struct {
	Accelerator  string `json:"accelerator"`
	InstanceType string `json:"instance_type"`
	Replicas     uint32 `json:"replicas"`
}

// Scheduler contains the v0.1 continuous-batching controls.
type Scheduler struct {
	MaxNumBatchedTokens uint64 `json:"max_num_batched_tokens"`
	MaxSequences        uint32 `json:"max_sequences"`
}

// Workload points to a privacy-safe trace and defines tenant weights.
type Workload struct {
	Trace       string   `json:"trace"`
	ReplaySpeed float64  `json:"replay_speed"`
	Tenants     []Tenant `json:"tenants"`
}

// Tenant defines one workload class and its relative fairness weight.
type Tenant struct {
	Name   string `json:"name"`
	Weight uint32 `json:"weight"`
}

// Policies defines the mandatory SLO, fairness, cost, and uncertainty gates.
type Policies struct {
	TTFTP99Milliseconds            uint64  `json:"ttft_p99_milliseconds"`
	TPOTP99Milliseconds            uint64  `json:"tpot_p99_milliseconds"`
	MinimumFairnessIndex           float64 `json:"minimum_fairness_index"`
	MaximumCostPerMillionTokensUSD float64 `json:"maximum_cost_per_million_tokens_usd"`
	MaximumViolationProbability    float64 `json:"maximum_violation_probability"`
}

// Fault declares one bounded inference-specific fault campaign.
type Fault struct {
	Type            string   `json:"type"`
	Probability     float64  `json:"probability,omitempty"`
	DurationSeconds []uint32 `json:"duration_seconds,omitempty"`
	PromptTokens    []uint64 `json:"prompt_tokens,omitempty"`
}

// Budget limits a separately authorized run; it does not authorize external action.
type Budget struct {
	MaximumExperimentCostUSD float64 `json:"maximum_experiment_cost_usd"`
	MaximumGPUMinutes        uint32  `json:"maximum_gpu_minutes"`
}

// NewDocument returns a document initialized with the current schema.
func NewDocument() Document {
	return Document{Schema: Schema, SchemaVersion: CurrentSchemaVersion}
}
