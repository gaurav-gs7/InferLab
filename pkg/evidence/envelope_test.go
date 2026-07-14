package evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestEnvelopeRoundTripAndCanonicalDigest(t *testing.T) {
	t.Parallel()
	first := validEnvelope()
	second := validEnvelope()
	second.Metrics[0], second.Metrics[1] = second.Metrics[1], second.Metrics[0]
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Name != first.Name {
		t.Fatalf("decoded name = %q, want %q", decoded.Name, first.Name)
	}
	firstDigest, err := Digest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := Digest(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("equivalent envelopes produced %q and %q", firstDigest, secondDigest)
	}
}

func TestValidateEnvelope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		mutate  func(*Envelope)
		wantErr error
	}{
		{name: "valid"},
		{name: "unknown runtime in complete evidence", mutate: func(e *Envelope) { e.Runtime.Platform.DriverVersion = "" }, wantErr: ErrIncompleteEvidence},
		{name: "declared observed evidence", mutate: func(e *Envelope) { e.Runtime.Origin = OriginDeclared }, wantErr: ErrInvalidEnvelope},
		{name: "non-finite metric", mutate: func(e *Envelope) { e.Metrics[0].Value = math.Inf(1) }, wantErr: ErrInvalidEnvelope},
		{name: "ambiguous unit", mutate: func(e *Envelope) { e.Metrics[0].Unit = "ms" }, wantErr: ErrInvalidEnvelope},
		{name: "duplicate metric", mutate: func(e *Envelope) { e.Metrics[1] = e.Metrics[0] }, wantErr: ErrInvalidEnvelope},
		{name: "backwards timestamps", mutate: func(e *Envelope) { e.FinishedAt = "2026-07-14T11:00:00Z" }, wantErr: ErrInvalidEnvelope},
		{name: "derived without parents", mutate: func(e *Envelope) { e.Classification = ClassDerived }, wantErr: ErrInvalidEnvelope},
		{name: "mutable adapter revision", mutate: func(e *Envelope) { e.Source.AdapterRevision = "main" }, wantErr: ErrInvalidEnvelope},
		{name: "missing artifact", mutate: func(e *Envelope) { e.Artifacts = nil }, wantErr: ErrInvalidEnvelope},
		{name: "schema", mutate: func(e *Envelope) { e.Schema = "other" }, wantErr: ErrUnsupportedSchema},
		{name: "version", mutate: func(e *Envelope) { e.SchemaVersion = "2.0" }, wantErr: ErrUnsupportedVersion},
		{name: "name", mutate: func(e *Envelope) { e.Name = "Invalid" }, wantErr: ErrInvalidEnvelope},
		{name: "classification", mutate: func(e *Envelope) { e.Classification = "measured" }, wantErr: ErrInvalidEnvelope},
		{name: "completeness", mutate: func(e *Envelope) { e.Completeness = "done" }, wantErr: ErrInvalidEnvelope},
		{name: "source tool", mutate: func(e *Envelope) { e.Source.Tool = "GuideLLM" }, wantErr: ErrInvalidEnvelope},
		{name: "source version", mutate: func(e *Envelope) { e.Source.ToolVersion = "" }, wantErr: ErrInvalidEnvelope},
		{name: "mutable source version", mutate: func(e *Envelope) { e.Source.ToolVersion = "latest" }, wantErr: ErrInvalidEnvelope},
		{name: "runtime", mutate: func(e *Envelope) { e.Runtime.Origin = "guessed" }, wantErr: ErrInvalidSignature},
		{name: "workload", mutate: func(e *Envelope) { e.WorkloadDigest = "bad" }, wantErr: ErrInvalidEnvelope},
		{name: "attempt", mutate: func(e *Envelope) { e.Attempt = 0 }, wantErr: ErrInvalidEnvelope},
		{name: "started at", mutate: func(e *Envelope) { e.StartedAt = "today" }, wantErr: ErrInvalidEnvelope},
		{name: "missing finished at", mutate: func(e *Envelope) { e.FinishedAt = "" }, wantErr: ErrIncompleteEvidence},
		{name: "finished at", mutate: func(e *Envelope) { e.FinishedAt = "today" }, wantErr: ErrInvalidEnvelope},
		{name: "missing complete metrics", mutate: func(e *Envelope) { e.Metrics = nil }, wantErr: ErrIncompleteEvidence},
		{name: "metric name", mutate: func(e *Envelope) { e.Metrics[0].Name = "TTFT" }, wantErr: ErrInvalidEnvelope},
		{name: "probability range", mutate: func(e *Envelope) { e.Metrics[0].Unit = "probability"; e.Metrics[0].Value = 2 }, wantErr: ErrInvalidEnvelope},
		{name: "sample count", mutate: func(e *Envelope) { e.Metrics[0].SampleCount = 0 }, wantErr: ErrInvalidEnvelope},
		{name: "artifact name", mutate: func(e *Envelope) { e.Artifacts[0].Name = "Raw Report" }, wantErr: ErrInvalidEnvelope},
		{name: "artifact media", mutate: func(e *Envelope) { e.Artifacts[0].MediaType = "json" }, wantErr: ErrInvalidEnvelope},
		{name: "artifact digest", mutate: func(e *Envelope) { e.Artifacts[0].Digest = "bad" }, wantErr: ErrInvalidEnvelope},
		{name: "duplicate artifact", mutate: func(e *Envelope) { e.Artifacts = append(e.Artifacts, e.Artifacts[0]) }, wantErr: ErrInvalidEnvelope},
		{name: "parent digest", mutate: func(e *Envelope) { e.ParentDigests = []string{"bad"} }, wantErr: ErrInvalidEnvelope},
		{name: "duplicate parent", mutate: func(e *Envelope) {
			e.ParentDigests = []string{"sha256:" + strings.Repeat("b", 64), "sha256:" + strings.Repeat("b", 64)}
		}, wantErr: ErrInvalidEnvelope},
		{name: "transformation", mutate: func(e *Envelope) { e.Transformation = &Transformation{Name: "Invalid", Revision: "main"} }, wantErr: ErrInvalidEnvelope},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			envelope := validEnvelope()
			if tt.mutate != nil {
				tt.mutate(&envelope)
			}
			err := ValidateEnvelope(envelope)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestPartialAndDerivedEvidence(t *testing.T) {
	t.Parallel()
	partial := validEnvelope()
	partial.Completeness = CompletenessPartial
	partial.Runtime.Platform.DriverVersion = ""
	partial.FinishedAt = ""
	partial.Metrics = nil
	if err := ValidateEnvelope(partial); err != nil {
		t.Fatalf("partial evidence error = %v", err)
	}

	derived := validEnvelope()
	derived.Classification = ClassDerived
	derived.ParentDigests = []string{"sha256:" + strings.Repeat("b", 64)}
	derived.Transformation = &Transformation{Name: "p99-aggregator", Revision: strings.Repeat("c", 40)}
	if err := ValidateEnvelope(derived); err != nil {
		t.Fatalf("derived evidence error = %v", err)
	}
}

func TestDecodeRejectsUntrustedInput(t *testing.T) {
	t.Parallel()
	encoded, err := json.Marshal(validEnvelope())
	if err != nil {
		t.Fatal(err)
	}
	base := bytes.TrimSuffix(encoded, []byte("}"))
	unknown := append(bytes.Clone(base), []byte(`,"unexpected":true}`)...)
	duplicate := append(bytes.Clone(base), []byte(`,"name":"other"}`)...)
	tests := []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{name: "unknown field", input: unknown, wantErr: ErrInvalidEnvelope},
		{name: "duplicate field", input: duplicate, wantErr: ErrDuplicateField},
		{name: "trailing value", input: append(encoded, []byte("\n{}")...), wantErr: ErrInvalidEnvelope},
		{name: "too large", input: bytes.Repeat([]byte("x"), MaxDocumentBytes+1), wantErr: ErrDocumentTooLarge},
		{name: "empty", input: nil, wantErr: ErrInvalidEnvelope},
		{name: "too deeply nested", input: append(bytes.Repeat([]byte("["), maxJSONDepth+2), append([]byte("0"), bytes.Repeat([]byte("]"), maxJSONDepth+2)...)...), wantErr: ErrNestingLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode(bytes.NewReader(tt.input))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
