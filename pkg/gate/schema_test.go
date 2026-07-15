package gate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishedGateSchemasAreValidJSON(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"evaluation.schema.json", "result.schema.json", "counterexample.schema.json"} {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(filepath.Join("..", "..", "schemas", "gate", "v1", name))
			if err != nil {
				t.Fatal(err)
			}
			var schema map[string]any
			if err := json.Unmarshal(data, &schema); err != nil {
				t.Fatalf("schema is invalid JSON: %v", err)
			}
			if schema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
				t.Fatalf("unexpected dialect: %v", schema["$schema"])
			}
		})
	}
}
