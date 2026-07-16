package adapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/gaurav-gs7/InferLab/internal/strictjson"
	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

func TestDecodeInputRejectsUntrustedJSON(t *testing.T) {
	t.Parallel()
	encoded, err := json.Marshal(testInput(GuideLLMAdapterName))
	if err != nil {
		t.Fatal(err)
	}
	base := bytes.TrimSuffix(encoded, []byte("}"))
	tests := []struct {
		name    string
		data    []byte
		wantErr error
	}{
		{name: "valid", data: encoded},
		{name: "unknown", data: append(bytes.Clone(base), []byte(`,"extra":true}`)...), wantErr: ErrInvalidInput},
		{name: "duplicate", data: append(bytes.Clone(base), []byte(`,"name":"duplicate"}`)...), wantErr: strictjson.ErrDuplicateField},
		{name: "trailing", data: append(bytes.Clone(encoded), []byte("\n{}")...), wantErr: ErrInvalidInput},
		{name: "too large", data: bytes.Repeat([]byte("x"), MaxInputBytes+1), wantErr: strictjson.ErrTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, gotErr := DecodeInput(bytes.NewReader(tt.data))
			if tt.wantErr == nil && gotErr != nil {
				t.Fatal(gotErr)
			}
			if tt.wantErr != nil && !errors.Is(gotErr, tt.wantErr) && !strings.Contains(gotErr.Error(), tt.wantErr.Error()) {
				t.Fatalf("error = %v, want %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestValidateInputRejectsIncompleteBoundaryProvenance(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*Input)
	}{
		{name: "invalid start", mutate: func(input *Input) { input.StartedAt = "today" }},
		{name: "missing complete finish", mutate: func(input *Input) { input.FinishedAt = "" }},
		{name: "backwards time", mutate: func(input *Input) { input.FinishedAt = "2026-07-15T07:59:59Z" }},
		{name: "unknown complete runtime", mutate: func(input *Input) { input.Runtime.Platform.DriverVersion = "" }},
		{name: "mutable producer range", mutate: func(input *Input) { input.Producer.ToolVersion = "0.6.x" }},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			input := testInput(GuideLLMAdapterName)
			test.mutate(&input)
			if err := ValidateInput(input); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("ValidateInput() error = %v, want %v", err, ErrInvalidInput)
			}
		})
	}

	partial := testInput(GuideLLMAdapterName)
	partial.Completeness = evidence.CompletenessPartial
	partial.FinishedAt = ""
	partial.Runtime.Platform.DriverVersion = ""
	if err := ValidateInput(partial); err != nil {
		t.Fatalf("partial input should preserve explicit unknown identity: %v", err)
	}
}

func TestProtocolAndCapabilitiesAreResourceAndLogSafe(t *testing.T) {
	t.Parallel()
	capabilities := Builtins()[0].Capabilities()
	capabilities.Metrics = make([]MetricCapability, maxCapabilityMetrics+1)
	if err := ValidateCapabilities(capabilities); !errors.Is(err, ErrInvalidCapabilities) {
		t.Fatalf("oversized capabilities error = %v, want %v", err, ErrInvalidCapabilities)
	}

	response := Response{
		Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "request-1",
		Failure: &Failure{Code: "producer-failed", Message: "line one\nforged line"},
	}
	if err := ValidateResponse(response); !errors.Is(err, ErrProtocol) {
		t.Fatalf("control-character failure error = %v, want %v", err, ErrProtocol)
	}
}
