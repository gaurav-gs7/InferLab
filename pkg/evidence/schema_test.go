package evidence

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishedExamplesAndSchemas(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	evidenceBytes, err := os.ReadFile(filepath.Join(root, "examples", "guidellm-observed-evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(bytes.NewReader(evidenceBytes)); err != nil {
		t.Fatalf("published evidence example does not validate: %v", err)
	}
	runtimeBytes, err := os.ReadFile(filepath.Join(root, "examples", "runtime-signature-l4-vllm.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeRuntimeSignature(bytes.NewReader(runtimeBytes)); err != nil {
		t.Fatalf("published runtime example does not validate: %v", err)
	}

	checks := []struct {
		path       string
		wantSchema string
	}{
		{path: "envelope.schema.json", wantSchema: EnvelopeSchema},
		{path: "runtime-signature.schema.json", wantSchema: SignatureSchema},
	}
	for _, check := range checks {
		data, err := os.ReadFile(filepath.Join(root, "schemas", "evidence", "v1", check.path))
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Properties map[string]struct {
				Const string `json:"const"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(data, &schema); err != nil {
			t.Fatalf("%s is invalid JSON: %v", check.path, err)
		}
		if got := schema.Properties["schema"].Const; got != check.wantSchema {
			t.Errorf("%s schema const = %q, want %q", check.path, got, check.wantSchema)
		}
		if got := schema.Properties["schema_version"].Const; got != CurrentSchemaVersion {
			t.Errorf("%s version const = %q, want %q", check.path, got, CurrentSchemaVersion)
		}
	}
}

func TestDecodeRuntimeSignatureRejectsUnknownFields(t *testing.T) {
	t.Parallel()
	encoded, err := json.Marshal(validRuntimeSignature(OriginObserved))
	if err != nil {
		t.Fatal(err)
	}
	unknown := append(bytes.TrimSuffix(encoded, []byte("}")), []byte(`,"unexpected":true}`)...)
	if _, err := DecodeRuntimeSignature(bytes.NewReader(unknown)); err == nil {
		t.Fatal("DecodeRuntimeSignature() accepted an unknown field")
	}
}
