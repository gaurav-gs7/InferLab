package gate

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/gaurav-gs7/InferLab/internal/strictjson"
	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

func TestDecodeEvaluationRejectsUntrustedJSON(t *testing.T) {
	t.Parallel()
	encoded, err := json.Marshal(validGateEvaluation())
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
		{name: "unknown", data: append(bytes.Clone(base), []byte(`,"unknown":true}`)...), wantErr: ErrInvalidEvaluation},
		{name: "duplicate", data: append(bytes.Clone(base), []byte(`,"name":"duplicate"}`)...), wantErr: strictjson.ErrDuplicateField},
		{name: "trailing", data: append(bytes.Clone(encoded), []byte("\n{}")...), wantErr: ErrInvalidEvaluation},
		{name: "too large", data: bytes.Repeat([]byte("x"), MaxDocumentBytes+1), wantErr: strictjson.ErrTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, gotErr := DecodeEvaluation(bytes.NewReader(tt.data))
			if tt.wantErr == nil && gotErr != nil {
				t.Fatal(gotErr)
			}
			if tt.wantErr != nil && !errors.Is(gotErr, tt.wantErr) {
				t.Fatalf("error = %v, want %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestDecodeResultRoundTrip(t *testing.T) {
	t.Parallel()
	result, err := Evaluate(validGateEvaluation())
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := CanonicalResultJSON(result)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeResult(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Decision != DecisionPass || decoded.Evaluation != result.Evaluation {
		t.Fatalf("unexpected decoded result: %+v", decoded)
	}
}

func TestValidateResultRejectsBrokenDecisionClosure(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*Result)
	}{
		{name: "conclusive code mismatch", mutate: func(result *Result) { result.Findings[0].Code = CodeViolationRisk }},
		{name: "coverage admits rejection", mutate: func(result *Result) {
			result.Admissions[0].Status = AdmissionRejected
			result.Admissions[0].Codes = []FindingCode{CodeStaleEvidence}
		}},
		{name: "forged compatibility", mutate: func(result *Result) {
			result.Admissions[0].Compatibility.Differences = []evidence.Dimension{evidence.DimensionDriverVersion}
		}},
		{name: "missing evaluation claim", mutate: func(result *Result) {
			for i, node := range result.Graph {
				if node.Kind == evidence.NodeClaim {
					result.Graph = append(result.Graph[:i], result.Graph[i+1:]...)
					return
				}
			}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := Evaluate(validGateEvaluation())
			if err != nil {
				t.Fatal(err)
			}
			tt.mutate(&result)
			if err := ValidateResult(result); !errors.Is(err, ErrInvalidResult) {
				t.Fatalf("error = %v, want invalid result", err)
			}
		})
	}
}
