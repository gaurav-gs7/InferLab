package trace

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestPublishedSchemaMatchesRecordPrivacyBoundary(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile("../../schemas/trace/v1/record.schema.json")
	if err != nil {
		t.Fatalf("read JSON Schema: %v", err)
	}
	var schema struct {
		AdditionalProperties bool                       `json:"additionalProperties"`
		Properties           map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Fatalf("parse JSON Schema: %v", err)
	}
	if schema.AdditionalProperties {
		t.Fatal("writer JSON Schema must reject unrecognized fields")
	}

	recordType := reflect.TypeFor[Record]()
	if len(schema.Properties) != recordType.NumField() {
		t.Fatalf("schema has %d properties, Record has %d fields", len(schema.Properties), recordType.NumField())
	}
	for i := range recordType.NumField() {
		field := recordType.Field(i)
		name := strings.Split(field.Tag.Get("json"), ",")[0]
		if name == "" || sensitiveFieldName(name) {
			t.Fatalf("Record field %s has unsafe JSON name %q", field.Name, name)
		}
		if _, exists := schema.Properties[name]; !exists {
			t.Fatalf("JSON Schema has no property for Record field %q", name)
		}
	}
}
