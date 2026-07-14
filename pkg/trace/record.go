// Package trace defines InferLab's versioned, privacy-safe workload trace
// format and its bounded streaming codec.
package trace

const (
	// Schema identifies InferLab scheduling metadata records.
	Schema = "inferlab.trace"
	// CurrentSchemaVersion is the schema version emitted by this package.
	CurrentSchemaVersion = "1.0"
	// SupportedSchemaMajor is the major version this package can decode.
	SupportedSchemaMajor = 1
)

// Record is one privacy-safe scheduling observation. It intentionally has no
// field for raw prompts, messages, request bodies, generated text, or response
// bodies. Times and token units are encoded in field names.
type Record struct {
	Schema            string            `json:"schema"`
	SchemaVersion     string            `json:"schema_version"`
	Sequence          uint64            `json:"sequence"`
	ArrivalOffsetNS   int64             `json:"arrival_offset_ns"`
	RequestID         string            `json:"request_id"`
	TenantID          string            `json:"tenant_id"`
	Model             string            `json:"model"`
	InputTokens       uint64            `json:"input_tokens"`
	MaxOutputTokens   uint64            `json:"max_output_tokens"`
	OutputTokens      uint64            `json:"output_tokens,omitempty"`
	PrefixFingerprint string            `json:"prefix_fingerprint,omitempty"`
	Adapter           string            `json:"adapter,omitempty"`
	Priority          uint8             `json:"priority"`
	DeadlineMS        uint64            `json:"deadline_ms,omitempty"`
	ObservedTTFTMS    *float64          `json:"observed_ttft_ms,omitempty"`
	ObservedTPOTMS    *float64          `json:"observed_tpot_ms,omitempty"`
	SelectedEndpoint  string            `json:"selected_endpoint,omitempty"`
	Metadata          map[string]string `json:"metadata,omitempty"`
}

// NewRecord returns a record initialized with the schema emitted by this
// package. Callers must populate the remaining required fields.
func NewRecord() Record {
	return Record{
		Schema:        Schema,
		SchemaVersion: CurrentSchemaVersion,
	}
}

// Limits bounds memory, input size, and semantic work performed by the trace
// codec. Zero-valued fields are replaced by DefaultLimits values.
type Limits struct {
	MaxRecordBytes        int
	MaxTraceBytes         int64
	MaxRecords            uint64
	MaxInputTokens        uint64
	MaxOutputTokens       uint64
	MaxTotalTokens        uint64
	MaxMetadataEntries    int
	MaxMetadataKeyBytes   int
	MaxMetadataValueBytes int
	MaxNestingDepth       int
}

// DefaultLimits returns conservative production defaults. Callers processing
// untrusted traces should lower them when their workload envelope is smaller.
func DefaultLimits() Limits {
	return Limits{
		MaxRecordBytes:        1 << 20,
		MaxTraceBytes:         4 << 30,
		MaxRecords:            1_000_000,
		MaxInputTokens:        4_000_000,
		MaxOutputTokens:       1_000_000,
		MaxTotalTokens:        5_000_000,
		MaxMetadataEntries:    32,
		MaxMetadataKeyBytes:   64,
		MaxMetadataValueBytes: 512,
		MaxNestingDepth:       16,
	}
}

func normalizeLimits(limits Limits) Limits {
	defaults := DefaultLimits()
	if limits.MaxRecordBytes <= 0 {
		limits.MaxRecordBytes = defaults.MaxRecordBytes
	}
	if limits.MaxTraceBytes <= 0 {
		limits.MaxTraceBytes = defaults.MaxTraceBytes
	}
	if limits.MaxRecords == 0 {
		limits.MaxRecords = defaults.MaxRecords
	}
	if limits.MaxInputTokens == 0 {
		limits.MaxInputTokens = defaults.MaxInputTokens
	}
	if limits.MaxOutputTokens == 0 {
		limits.MaxOutputTokens = defaults.MaxOutputTokens
	}
	if limits.MaxTotalTokens == 0 {
		limits.MaxTotalTokens = defaults.MaxTotalTokens
	}
	if limits.MaxMetadataEntries <= 0 {
		limits.MaxMetadataEntries = defaults.MaxMetadataEntries
	}
	if limits.MaxMetadataKeyBytes <= 0 {
		limits.MaxMetadataKeyBytes = defaults.MaxMetadataKeyBytes
	}
	if limits.MaxMetadataValueBytes <= 0 {
		limits.MaxMetadataValueBytes = defaults.MaxMetadataValueBytes
	}
	if limits.MaxNestingDepth <= 0 {
		limits.MaxNestingDepth = defaults.MaxNestingDepth
	}
	return limits
}
