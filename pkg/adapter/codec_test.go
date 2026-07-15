package adapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/gaurav-gs7/InferLab/internal/strictjson"
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
