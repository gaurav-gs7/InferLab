package change

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDecodeRoundTrip(t *testing.T) {
	t.Parallel()
	document := validDocument()
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Decode(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("Decode() error = %v", err)
	}
	if got.Name != document.Name || got.Candidate.Scheduler != document.Candidate.Scheduler {
		t.Fatalf("Decode() = %#v, want %#v", got, document)
	}
}

func TestDecodeRejectsUntrustedInput(t *testing.T) {
	t.Parallel()
	document := validDocument()
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	unknown := append(bytes.TrimSuffix(encoded, []byte("}")), []byte(`,"unexpected":true}`)...)

	tests := []struct {
		name    string
		input   []byte
		wantErr error
	}{
		{name: "unknown field", input: unknown, wantErr: ErrInvalidDocument},
		{name: "trailing value", input: append(encoded, []byte("\n{}")...), wantErr: ErrInvalidDocument},
		{name: "too large", input: bytes.Repeat([]byte("x"), MaxDocumentBytes+1), wantErr: ErrDocumentTooLarge},
		{name: "empty", input: nil, wantErr: ErrInvalidDocument},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := Decode(bytes.NewReader(tt.input))
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Decode() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
	if _, err := Decode(nil); !errors.Is(err, ErrInvalidDocument) {
		t.Fatalf("Decode(nil) error = %v, want %v", err, ErrInvalidDocument)
	}
}

func TestDigestCanonicalizesUnorderedCollections(t *testing.T) {
	t.Parallel()
	first := validDocument()
	second := validDocument()
	second.Workload.Tenants[0], second.Workload.Tenants[1] = second.Workload.Tenants[1], second.Workload.Tenants[0]
	second.Faults[0], second.Faults[1] = second.Faults[1], second.Faults[0]

	firstDigest, err := Digest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := Digest(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("Digest() = %q and %q for equivalent documents", firstDigest, secondDigest)
	}
	if !strings.HasPrefix(firstDigest, "sha256:") || len(firstDigest) != len("sha256:")+64 {
		t.Fatalf("Digest() = %q, want sha256-prefixed digest", firstDigest)
	}
}

func TestDigestChangesWithConfiguration(t *testing.T) {
	t.Parallel()
	first := validDocument()
	second := validDocument()
	second.Candidate.Scheduler.MaxSequences++
	firstDigest, err := Digest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := Digest(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest == secondDigest {
		t.Fatalf("Digest() did not change after candidate mutation")
	}
}
