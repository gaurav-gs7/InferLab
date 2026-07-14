package change

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishedExampleAndSchema(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..")
	example, err := os.Open(filepath.Join(root, "examples", "qwen-vllm-batching-change.json"))
	if err != nil {
		t.Fatal(err)
	}
	defer example.Close()
	document, err := Decode(example)
	if err != nil {
		t.Fatalf("published example does not validate: %v", err)
	}
	if document.Schema != Schema || document.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("published example identifies %s %s", document.Schema, document.SchemaVersion)
	}

	schemaBytes, err := os.ReadFile(filepath.Join(root, "schemas", "change", "v1", "inference-change.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Properties map[string]struct {
			Const string `json:"const"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("schema is invalid JSON: %v", err)
	}
	if got := schema.Properties["schema"].Const; got != Schema {
		t.Errorf("schema const = %q, want %q", got, Schema)
	}
	if got := schema.Properties["schema_version"].Const; got != CurrentSchemaVersion {
		t.Errorf("schema version const = %q, want %q", got, CurrentSchemaVersion)
	}
}
