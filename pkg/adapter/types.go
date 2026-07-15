// Package adapter defines the versioned boundary between evidence producers
// and InferLab's source-neutral evidence contract.
package adapter

import (
	"encoding/json"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

const (
	ProtocolSchema         = "inferlab.adapter-protocol"
	NormalizedReportSchema = "inferlab.normalized-report"
	CurrentVersion         = "1.0"
	MaxInputBytes          = 8 << 20
	DefaultMaxOutputBytes  = 8 << 20
)

type Operation string

const (
	OperationCapabilities Operation = "capabilities"
	OperationNormalize    Operation = "normalize"
)

// Adapter normalizes an exact, declared producer contract. Implementations
// may be in-process or hosted behind Runner's out-of-process protocol.
type Adapter interface {
	Capabilities() Capabilities
	Normalize(input Input) (NormalizedReport, error)
}

type AdapterIdentity struct {
	Name     string `json:"name"`
	Revision string `json:"revision"`
}

type ProducerIdentity struct {
	Tool         string `json:"tool"`
	ToolVersion  string `json:"tool_version"`
	ReportSchema string `json:"report_schema"`
}

type Capabilities struct {
	Schema          string                    `json:"schema"`
	SchemaVersion   string                    `json:"schema_version"`
	Adapter         AdapterIdentity           `json:"adapter"`
	Producer        ProducerIdentity          `json:"producer"`
	Classifications []evidence.Classification `json:"classifications"`
	Metrics         []MetricCapability        `json:"metrics"`
}

type MetricCapability struct {
	SourceName       string `json:"source_name"`
	SourceDefinition string `json:"source_definition"`
	SourceUnit       string `json:"source_unit"`
	NormalizedName   string `json:"normalized_name"`
	Semantics        string `json:"semantics"`
	NormalizedUnit   string `json:"normalized_unit"`
}

// Input binds an untrusted producer report to exact execution provenance.
// Report is the exact JSON value whose digest is retained as an artifact.
type Input struct {
	Name           string                    `json:"name"`
	Producer       ProducerIdentity          `json:"producer"`
	Classification evidence.Classification   `json:"classification"`
	Completeness   evidence.Completeness     `json:"completeness"`
	Runtime        evidence.RuntimeSignature `json:"runtime"`
	WorkloadDigest string                    `json:"workload_digest"`
	Attempt        uint32                    `json:"attempt"`
	StartedAt      string                    `json:"started_at"`
	FinishedAt     string                    `json:"finished_at,omitempty"`
	Report         json.RawMessage           `json:"report"`
}

// OriginalMetric preserves the producer's value, unit, and definition.
type OriginalMetric struct {
	Name        string  `json:"name"`
	Definition  string  `json:"definition"`
	Value       float64 `json:"value"`
	Unit        string  `json:"unit"`
	SampleCount uint64  `json:"sample_count"`
}

// MappingRecord makes every unit and semantic conversion reviewable.
type MappingRecord struct {
	SourceName       string  `json:"source_name"`
	SourceDefinition string  `json:"source_definition"`
	SourceValue      float64 `json:"source_value"`
	SourceUnit       string  `json:"source_unit"`
	NormalizedName   string  `json:"normalized_name"`
	Semantics        string  `json:"semantics"`
	NormalizedValue  float64 `json:"normalized_value"`
	NormalizedUnit   string  `json:"normalized_unit"`
	Scale            float64 `json:"scale"`
	Offset           float64 `json:"offset"`
}

// NormalizedReport is the lossless normalization artifact. Envelope is the
// source-neutral projection; Originals and Mappings prevent provenance loss.
type NormalizedReport struct {
	Schema        string            `json:"schema"`
	SchemaVersion string            `json:"schema_version"`
	Adapter       AdapterIdentity   `json:"adapter"`
	InputDigest   string            `json:"input_digest"`
	Originals     []OriginalMetric  `json:"originals"`
	Mappings      []MappingRecord   `json:"mappings"`
	Envelope      evidence.Envelope `json:"envelope"`
}

type Request struct {
	Schema        string    `json:"schema"`
	SchemaVersion string    `json:"schema_version"`
	RequestID     string    `json:"request_id"`
	Operation     Operation `json:"operation"`
	Input         *Input    `json:"input,omitempty"`
}

type Response struct {
	Schema        string            `json:"schema"`
	SchemaVersion string            `json:"schema_version"`
	RequestID     string            `json:"request_id"`
	Capabilities  *Capabilities     `json:"capabilities,omitempty"`
	Report        *NormalizedReport `json:"report,omitempty"`
	Failure       *Failure          `json:"failure,omitempty"`
}

type Failure struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
