package adapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestPublishedSchemasAreValidJSONAndPinV1(t *testing.T) {
	t.Parallel()
	files := []string{"input.schema.json", "capabilities.schema.json", "normalized-report.schema.json", "protocol.schema.json"}
	for _, file := range files {
		file := file
		t.Run(file, func(t *testing.T) {
			t.Parallel()
			data, err := os.ReadFile(filepath.Join("..", "..", "schemas", "adapter", "v1", file))
			if err != nil {
				t.Fatal(err)
			}
			var document map[string]any
			if err := json.Unmarshal(data, &document); err != nil {
				t.Fatalf("schema is invalid JSON: %v", err)
			}
			if document["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
				t.Fatalf("unexpected schema dialect: %v", document["$schema"])
			}
		})
	}
}
