// Package evidence defines source-neutral runtime identity and evidence
// contracts for evaluating an inference change.
package evidence

const (
	// SignatureSchema identifies runtime-signature documents.
	SignatureSchema = "inferlab.runtime-signature"
	// EnvelopeSchema identifies evidence-envelope documents.
	EnvelopeSchema = "inferlab.evidence"
	// CurrentSchemaVersion is the schema version emitted by this package.
	CurrentSchemaVersion = "1.0"
	// MaxDocumentBytes bounds untrusted evidence documents.
	MaxDocumentBytes = 4 << 20
)

// Origin distinguishes intended configuration from identity observed at run time.
type Origin string

const (
	OriginDeclared Origin = "declared"
	OriginObserved Origin = "observed"
)

// RuntimeSignature captures every material identity dimension used by v0.1.
// Empty material fields mean unknown; they never mean wildcard.
type RuntimeSignature struct {
	Schema        string            `json:"schema"`
	SchemaVersion string            `json:"schema_version"`
	Origin        Origin            `json:"origin"`
	Model         ModelIdentity     `json:"model"`
	Engine        EngineIdentity    `json:"engine"`
	Platform      PlatformIdentity  `json:"platform"`
	Scheduler     SchedulerIdentity `json:"scheduler"`
	Kernels       []KernelIdentity  `json:"kernels"`
}

type ModelIdentity struct {
	ID                       string `json:"id,omitempty"`
	Revision                 string `json:"revision,omitempty"`
	TokenizerID              string `json:"tokenizer_id,omitempty"`
	TokenizerRevision        string `json:"tokenizer_revision,omitempty"`
	Quantization             string `json:"quantization,omitempty"`
	QuantizationConfigDigest string `json:"quantization_config_digest,omitempty"`
}

type EngineIdentity struct {
	Name           string `json:"name,omitempty"`
	Revision       string `json:"revision,omitempty"`
	ContainerImage string `json:"container_image,omitempty"`
}

type PlatformIdentity struct {
	CUDAVersion   string `json:"cuda_version,omitempty"`
	DriverVersion string `json:"driver_version,omitempty"`
	GPUSKU        string `json:"gpu_sku,omitempty"`
	GPUCount      uint32 `json:"gpu_count,omitempty"`
	Topology      string `json:"topology,omitempty"`
}

type SchedulerIdentity struct {
	Name         string `json:"name,omitempty"`
	ConfigDigest string `json:"config_digest,omitempty"`
}

type KernelIdentity struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	ConfigDigest string `json:"config_digest"`
}

// NewRuntimeSignature returns a signature initialized to the current schema.
func NewRuntimeSignature(origin Origin) RuntimeSignature {
	return RuntimeSignature{
		Schema:        SignatureSchema,
		SchemaVersion: CurrentSchemaVersion,
		Origin:        origin,
		Kernels:       []KernelIdentity{},
	}
}

type Classification string

const (
	ClassObserved  Classification = "observed"
	ClassPredicted Classification = "predicted"
	ClassDerived   Classification = "derived"
	ClassAsserted  Classification = "asserted"
)

type Completeness string

const (
	CompletenessComplete Completeness = "complete"
	CompletenessPartial  Completeness = "partial"
)

// Envelope binds metrics and artifacts to their producer, runtime, workload,
// attempt, and transformation provenance.
type Envelope struct {
	Schema         string           `json:"schema"`
	SchemaVersion  string           `json:"schema_version"`
	Name           string           `json:"name"`
	Classification Classification   `json:"classification"`
	Completeness   Completeness     `json:"completeness"`
	Source         Source           `json:"source"`
	Runtime        RuntimeSignature `json:"runtime"`
	WorkloadDigest string           `json:"workload_digest"`
	Attempt        uint32           `json:"attempt"`
	StartedAt      string           `json:"started_at"`
	FinishedAt     string           `json:"finished_at,omitempty"`
	Metrics        []Metric         `json:"metrics,omitempty"`
	Artifacts      []Artifact       `json:"artifacts"`
	ParentDigests  []string         `json:"parent_digests,omitempty"`
	Transformation *Transformation  `json:"transformation,omitempty"`
}

type Source struct {
	Tool            string `json:"tool"`
	ToolVersion     string `json:"tool_version"`
	ReportSchema    string `json:"report_schema"`
	Adapter         string `json:"adapter"`
	AdapterRevision string `json:"adapter_revision"`
}

type Metric struct {
	Name        string  `json:"name"`
	Semantics   string  `json:"semantics"`
	Value       float64 `json:"value"`
	Unit        string  `json:"unit"`
	SampleCount uint64  `json:"sample_count"`
}

type Artifact struct {
	Name      string `json:"name"`
	MediaType string `json:"media_type"`
	Digest    string `json:"digest"`
}

type Transformation struct {
	Name     string `json:"name"`
	Revision string `json:"revision"`
}

// NewEnvelope returns an evidence envelope initialized to the current schema.
func NewEnvelope() Envelope {
	return Envelope{Schema: EnvelopeSchema, SchemaVersion: CurrentSchemaVersion}
}
