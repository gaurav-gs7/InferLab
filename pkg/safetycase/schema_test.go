package safetycase

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishedSchemasAreValidJSON(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"descriptor", "manifest", "signature"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join("..", "..", "schemas", "safety-case", "v1", name+".schema.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var schema map[string]any
			if err := json.Unmarshal(data, &schema); err != nil {
				t.Fatalf("invalid JSON Schema: %v", err)
			}
			if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
				t.Fatalf("unexpected JSON Schema dialect: %v", schema["$schema"])
			}
		})
	}
}
