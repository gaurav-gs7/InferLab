package adapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

type conformanceAdapter struct {
	capabilities Capabilities
	normalize    func(Input) (NormalizedReport, error)
}

func (adapter conformanceAdapter) Capabilities() Capabilities { return adapter.capabilities }
func (adapter conformanceAdapter) Normalize(input Input) (NormalizedReport, error) {
	return adapter.normalize(input)
}

func TestValidateCapabilitiesBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Capabilities)
	}{
		{name: "schema", mutate: func(value *Capabilities) { value.Schema = "other" }},
		{name: "adapter name", mutate: func(value *Capabilities) { value.Adapter.Name = "Bad Name" }},
		{name: "adapter revision", mutate: func(value *Capabilities) { value.Adapter.Revision = "latest" }},
		{name: "producer", mutate: func(value *Capabilities) { value.Producer.ReportSchema = "latest" }},
		{name: "empty classifications", mutate: func(value *Capabilities) { value.Classifications = nil }},
		{name: "empty metrics", mutate: func(value *Capabilities) { value.Metrics = nil }},
		{name: "unsupported classification", mutate: func(value *Capabilities) { value.Classifications[0] = evidence.Classification("synthetic") }},
		{name: "duplicate classification", mutate: func(value *Capabilities) {
			value.Classifications = append(value.Classifications, value.Classifications[0])
		}},
		{name: "invalid metric name", mutate: func(value *Capabilities) { value.Metrics[0].SourceName = "Bad Name" }},
		{name: "invalid source unit", mutate: func(value *Capabilities) { value.Metrics[0].SourceUnit = "" }},
		{name: "invalid normalized unit", mutate: func(value *Capabilities) { value.Metrics[0].NormalizedUnit = "\x00" }},
		{name: "duplicate source", mutate: func(value *Capabilities) {
			value.Metrics[1] = value.Metrics[0]
			value.Metrics[1].NormalizedName = "unique-target"
			value.Metrics[1].Semantics = "unique-semantics"
		}},
		{name: "duplicate target", mutate: func(value *Capabilities) {
			value.Metrics[1].NormalizedName = value.Metrics[0].NormalizedName
			value.Metrics[1].Semantics = value.Metrics[0].Semantics
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			capabilities := Builtins()[0].Capabilities()
			tt.mutate(&capabilities)
			if err := ValidateCapabilities(capabilities); !errors.Is(err, ErrInvalidCapabilities) {
				t.Fatalf("ValidateCapabilities() error = %v, want %v", err, ErrInvalidCapabilities)
			}
		})
	}
}

func TestValidateInputBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Input)
	}{
		{name: "name", mutate: func(value *Input) { value.Name = "Bad Name" }},
		{name: "classification", mutate: func(value *Input) { value.Classification = evidence.Classification("synthetic") }},
		{name: "completeness", mutate: func(value *Input) { value.Completeness = evidence.Completeness("unknown") }},
		{name: "runtime", mutate: func(value *Input) { value.Runtime.Schema = "other" }},
		{name: "observed with declared runtime", mutate: func(value *Input) { value.Runtime.Origin = evidence.OriginDeclared }},
		{name: "predicted with observed runtime", mutate: func(value *Input) { value.Classification = evidence.ClassPredicted }},
		{name: "finished time", mutate: func(value *Input) { value.FinishedAt = "later" }},
		{name: "workload digest", mutate: func(value *Input) { value.WorkloadDigest = "sha256:bad" }},
		{name: "attempt", mutate: func(value *Input) { value.Attempt = 0 }},
		{name: "empty report", mutate: func(value *Input) { value.Report = nil }},
		{name: "malformed report", mutate: func(value *Input) { value.Report = json.RawMessage(`{"bad":`) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := testInput(GuideLLMAdapterName)
			tt.mutate(&input)
			if err := ValidateInput(input); err == nil {
				t.Fatal("ValidateInput() accepted invalid input")
			}
		})
	}
}

func TestValidateNormalizedReportBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*NormalizedReport)
	}{
		{name: "schema", mutate: func(value *NormalizedReport) { value.Schema = "other" }},
		{name: "identity", mutate: func(value *NormalizedReport) { value.InputDigest = "bad" }},
		{name: "empty originals", mutate: func(value *NormalizedReport) { value.Originals = nil }},
		{name: "unequal lengths", mutate: func(value *NormalizedReport) { value.Mappings = value.Mappings[:1] }},
		{name: "invalid envelope", mutate: func(value *NormalizedReport) { value.Envelope.Schema = "other" }},
		{name: "source identity", mutate: func(value *NormalizedReport) { value.Envelope.Source.Adapter = "different" }},
		{name: "raw artifact count", mutate: func(value *NormalizedReport) { value.Envelope.Artifacts = nil }},
		{name: "raw artifact name", mutate: func(value *NormalizedReport) { value.Envelope.Artifacts[0].Name = "different" }},
		{name: "invalid original", mutate: func(value *NormalizedReport) { value.Originals[0].Value = math.NaN() }},
		{name: "duplicate original", mutate: func(value *NormalizedReport) { value.Originals[1] = value.Originals[0] }},
		{name: "nonfinite mapping", mutate: func(value *NormalizedReport) { value.Mappings[0].Scale = math.Inf(1) }},
		{name: "missing original", mutate: func(value *NormalizedReport) { value.Mappings[0].SourceDefinition = "different" }},
		{name: "source value", mutate: func(value *NormalizedReport) { value.Mappings[0].SourceValue++ }},
		{name: "missing normalized metric", mutate: func(value *NormalizedReport) { value.Mappings[0].Semantics = "different" }},
		{name: "normalized unit", mutate: func(value *NormalizedReport) { value.Mappings[0].NormalizedUnit = "different" }},
		{name: "conversion", mutate: func(value *NormalizedReport) { value.Mappings[0].Offset++ }},
		{name: "duplicate mapping", mutate: func(value *NormalizedReport) {
			value.Originals[1].Value = value.Originals[0].Value
			value.Mappings[1].SourceValue = value.Originals[0].Value
			value.Mappings[1].NormalizedName = value.Mappings[0].NormalizedName
			value.Mappings[1].Semantics = value.Mappings[0].Semantics
			value.Mappings[1].NormalizedValue = value.Mappings[0].NormalizedValue
			value.Mappings[1].NormalizedUnit = value.Mappings[0].NormalizedUnit
			value.Mappings[1].Scale = value.Mappings[0].NormalizedValue / value.Mappings[1].SourceValue
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			implementation, _ := Builtin(GuideLLMAdapterName)
			report, err := implementation.Normalize(testInput(GuideLLMAdapterName))
			if err != nil {
				t.Fatal(err)
			}
			tt.mutate(&report)
			if err := ValidateNormalizedReport(report); !errors.Is(err, ErrInvalidReport) {
				t.Fatalf("ValidateNormalizedReport() error = %v, want %v", err, ErrInvalidReport)
			}
		})
	}
}

func TestProtocolCodecRoundTripsAndRejectsInvalidPayloads(t *testing.T) {
	t.Parallel()

	input := testInput(GuideLLMAdapterName)
	normalize := Request{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "normalize-1", Operation: OperationNormalize, Input: &input}
	capabilities := Request{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "capabilities-1", Operation: OperationCapabilities}
	for _, request := range []Request{normalize, capabilities} {
		encoded, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeRequest(bytes.NewReader(encoded)); err != nil {
			t.Fatalf("DecodeRequest() error = %v", err)
		}
	}

	implementation, _ := Builtin(GuideLLMAdapterName)
	report, err := implementation.Normalize(input)
	if err != nil {
		t.Fatal(err)
	}
	encodedReport, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeNormalizedReport(bytes.NewReader(encodedReport)); err != nil {
		t.Fatalf("DecodeNormalizedReport() error = %v", err)
	}

	validCapabilities := implementation.Capabilities()
	responses := []Response{
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "response-1", Capabilities: &validCapabilities},
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "response-2", Report: &report},
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "response-3", Failure: &Failure{Code: "producer-failed", Message: "safe diagnostic"}},
	}
	for _, response := range responses {
		encoded, err := json.Marshal(response)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeResponse(bytes.NewReader(encoded), 0); err != nil {
			t.Fatalf("DecodeResponse() error = %v", err)
		}
	}

	invalidRequests := []Request{
		{},
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "request-1", Operation: OperationCapabilities, Input: &input},
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "request-1", Operation: OperationNormalize},
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "request-1", Operation: OperationNormalize, Input: &Input{}},
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "request-1", Operation: Operation("remove")},
	}
	for _, request := range invalidRequests {
		if err := ValidateRequest(request); !errors.Is(err, ErrProtocol) {
			t.Fatalf("ValidateRequest(%q) error = %v, want %v", request.Operation, err, ErrProtocol)
		}
	}

	invalidResponses := []Response{
		{},
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "response-1"},
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "response-1", Failure: &Failure{Code: "Bad Code", Message: "message"}},
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "response-1", Failure: &Failure{Code: "failed", Message: strings.Repeat("x", 1025)}},
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "response-1", Capabilities: &Capabilities{}},
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "response-1", Report: &NormalizedReport{}},
		{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "response-1", Capabilities: &validCapabilities, Failure: &Failure{Code: "failed", Message: "message"}},
	}
	for _, response := range invalidResponses {
		if err := ValidateResponse(response); !errors.Is(err, ErrProtocol) {
			t.Fatalf("ValidateResponse() error = %v, want %v", err, ErrProtocol)
		}
	}
}

func TestMappedAdapterConstructionAndNormalizationBoundaries(t *testing.T) {
	t.Parallel()

	identity := AdapterIdentity{Name: "test-adapter", Revision: strings.Repeat("a", 64)}
	producer := ProducerIdentity{Tool: "test-tool", ToolVersion: "1.0.0", ReportSchema: "test-report-v1"}
	mapping := metric("source", "source-definition-v1", "seconds", "target", "target-definition-v1", "milliseconds", 1000)
	for _, scale := range []float64{0, math.NaN(), math.Inf(1)} {
		candidate := mapping
		candidate.Scale = scale
		if _, err := NewPredictedAdapter(identity, producer, []Mapping{candidate}); !errors.Is(err, ErrInvalidCapabilities) {
			t.Fatalf("NewPredictedAdapter(scale=%v) error = %v", scale, err)
		}
	}
	candidate := mapping
	candidate.Offset = math.NaN()
	if _, err := NewPredictedAdapter(identity, producer, []Mapping{candidate}); !errors.Is(err, ErrInvalidCapabilities) {
		t.Fatalf("NewPredictedAdapter(offset=NaN) error = %v", err)
	}

	implementation, _ := Builtin(GuideLLMAdapterName)
	tests := []struct {
		name   string
		mutate func(*Input, *producerReport)
	}{
		{name: "invalid input", mutate: func(input *Input, _ *producerReport) { input.Name = "Bad Name" }},
		{name: "invalid report schema", mutate: func(_ *Input, report *producerReport) { report.Schema = "other" }},
		{name: "invalid run id", mutate: func(_ *Input, report *producerReport) { report.RunID = "Bad ID" }},
		{name: "empty metrics", mutate: func(input *Input, report *producerReport) {
			input.Completeness = evidence.CompletenessPartial
			report.Metrics = nil
		}},
		{name: "invalid metric", mutate: func(_ *Input, report *producerReport) { report.Metrics[0].SampleCount = 0 }},
		{name: "ratio out of range", mutate: func(_ *Input, report *producerReport) { report.Metrics[2].Value = 101 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := testInput(GuideLLMAdapterName)
			var raw producerReport
			if err := json.Unmarshal(input.Report, &raw); err != nil {
				t.Fatal(err)
			}
			tt.mutate(&input, &raw)
			encoded, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			input.Report = encoded
			if _, err := implementation.Normalize(input); err == nil {
				t.Fatal("Normalize() accepted invalid producer evidence")
			}
		})
	}
}

func TestConformanceRejectsBrokenImplementations(t *testing.T) {
	t.Parallel()

	implementation, _ := Builtin(GuideLLMAdapterName)
	input := testInput(GuideLLMAdapterName)
	base, err := implementation.Normalize(input)
	if err != nil {
		t.Fatal(err)
	}
	capabilities := implementation.Capabilities()
	if err := CheckConformance(nil, []Input{input}); !errors.Is(err, ErrInvalidCapabilities) {
		t.Fatalf("nil adapter error = %v", err)
	}
	if err := CheckConformance(conformanceAdapter{capabilities: Capabilities{}, normalize: implementation.Normalize}, []Input{input}); !errors.Is(err, ErrInvalidCapabilities) {
		t.Fatalf("invalid capabilities error = %v", err)
	}
	if err := CheckConformance(implementation, nil); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("empty fixtures error = %v", err)
	}
	mismatched := input
	mismatched.Producer.Tool = "another-tool"
	if err := CheckConformance(implementation, []Input{mismatched}); !errors.Is(err, ErrUnsupportedProducer) {
		t.Fatalf("producer mismatch error = %v", err)
	}

	tests := []struct {
		name      string
		normalize func(int, Input) (NormalizedReport, error)
	}{
		{name: "first normalize failure", normalize: func(_ int, _ Input) (NormalizedReport, error) { return NormalizedReport{}, errors.New("first failure") }},
		{name: "second normalize failure", normalize: func(call int, _ Input) (NormalizedReport, error) {
			if call == 2 {
				return NormalizedReport{}, errors.New("second failure")
			}
			return base, nil
		}},
		{name: "invalid canonical report", normalize: func(_ int, _ Input) (NormalizedReport, error) { return NormalizedReport{}, nil }},
		{name: "nondeterministic", normalize: func(call int, _ Input) (NormalizedReport, error) {
			report := base
			if call == 2 {
				report.InputDigest = "sha256:" + strings.Repeat("c", 64)
				report.Envelope.Artifacts[0].Digest = report.InputDigest
			}
			return report, nil
		}},
		{name: "classification drift", normalize: func(_ int, _ Input) (NormalizedReport, error) {
			report := base
			report.Envelope.Classification = evidence.ClassPredicted
			report.Envelope.Runtime.Origin = evidence.OriginDeclared
			return report, nil
		}},
		{name: "provenance drift", normalize: func(_ int, _ Input) (NormalizedReport, error) {
			report := base
			report.Envelope.Source.Tool = "different-tool"
			return report, nil
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			adapter := conformanceAdapter{capabilities: capabilities, normalize: func(input Input) (NormalizedReport, error) {
				calls++
				return tt.normalize(calls, input)
			}}
			if err := CheckConformance(adapter, []Input{input}); err == nil {
				t.Fatal("CheckConformance() accepted a broken adapter")
			}
		})
	}
}

func TestRegistryAndCapabilityEncoding(t *testing.T) {
	t.Parallel()

	implementation, ok := Builtin(GuideLLMAdapterName)
	if !ok {
		t.Fatal("Builtin() did not find the registered adapter")
	}
	if _, ok := Builtin("missing"); ok {
		t.Fatal("Builtin() found an unknown adapter")
	}
	encoded, err := MarshalCapabilities(implementation.Capabilities())
	if err != nil || !bytes.Contains(encoded, []byte(`"adapter"`)) {
		t.Fatalf("MarshalCapabilities() = %q, %v", encoded, err)
	}
	if _, err := MarshalCapabilities(Capabilities{}); !errors.Is(err, ErrInvalidCapabilities) {
		t.Fatalf("MarshalCapabilities() error = %v, want %v", err, ErrInvalidCapabilities)
	}
}
