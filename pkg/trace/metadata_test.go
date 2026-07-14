package trace

import (
	"errors"
	"reflect"
	"testing"
)

func TestMetadataPolicyFailClosedDefault(t *testing.T) {
	t.Parallel()

	policy, err := NewMetadataPolicy(MetadataPolicyOptions{})
	if err != nil {
		t.Fatalf("NewMetadataPolicy() error: %v", err)
	}
	filtered, dropped, err := policy.Filter(map[string]string{
		"region":        "ap-south-1",
		"traffic.class": "interactive",
	})
	if err != nil {
		t.Fatalf("Filter() error: %v", err)
	}
	if filtered != nil {
		t.Fatalf("Filter() = %v, want nil", filtered)
	}
	if want := []string{"region", "traffic.class"}; !reflect.DeepEqual(dropped, want) {
		t.Fatalf("Filter() dropped = %v, want %v", dropped, want)
	}
}

func TestMetadataPolicyAllowlist(t *testing.T) {
	t.Parallel()

	policy, err := NewMetadataPolicy(MetadataPolicyOptions{
		AllowedKeys: []string{"region", "traffic.class"},
	})
	if err != nil {
		t.Fatalf("NewMetadataPolicy() error: %v", err)
	}
	filtered, dropped, err := policy.Filter(map[string]string{
		"region":           "ap-south-1",
		"traffic.class":    "interactive",
		"unapproved.label": "discard-me",
	})
	if err != nil {
		t.Fatalf("Filter() error: %v", err)
	}
	wantFiltered := map[string]string{"region": "ap-south-1", "traffic.class": "interactive"}
	if !reflect.DeepEqual(filtered, wantFiltered) {
		t.Fatalf("Filter() = %v, want %v", filtered, wantFiltered)
	}
	if want := []string{"unapproved.label"}; !reflect.DeepEqual(dropped, want) {
		t.Fatalf("Filter() dropped = %v, want %v", dropped, want)
	}

	filtered["region"] = "mutated"
	again, _, err := policy.Filter(map[string]string{"region": "ap-south-1"})
	if err != nil {
		t.Fatalf("Filter() repeated error: %v", err)
	}
	if again["region"] != "ap-south-1" {
		t.Fatal("Filter() did not return a fresh map")
	}
}

func TestMetadataPolicyRejectsSensitiveAndInvalidConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		options MetadataPolicyOptions
		want    error
	}{
		{name: "prompt", options: MetadataPolicyOptions{AllowedKeys: []string{"user.prompt"}}, want: ErrSensitiveField},
		{name: "messages", options: MetadataPolicyOptions{AllowedKeys: []string{"messages"}}, want: ErrSensitiveField},
		{name: "body", options: MetadataPolicyOptions{AllowedKeys: []string{"request_body"}}, want: ErrSensitiveField},
		{name: "uppercase", options: MetadataPolicyOptions{AllowedKeys: []string{"Region"}}, want: ErrInvalidRecord},
		{name: "duplicate", options: MetadataPolicyOptions{AllowedKeys: []string{"region", "region"}}, want: ErrInvalidRecord},
		{name: "too many", options: MetadataPolicyOptions{AllowedKeys: []string{"region", "zone"}, MaxEntries: 1}, want: ErrMetadataLimit},
		{name: "invalid scan bound", options: MetadataPolicyOptions{AllowedKeys: []string{"region"}, MaxEntries: 2, MaxInputEntries: 1}, want: ErrMetadataLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if _, err := NewMetadataPolicy(tt.options); !errors.Is(err, tt.want) {
				t.Fatalf("NewMetadataPolicy() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestMetadataPolicyBoundsInputScan(t *testing.T) {
	t.Parallel()

	policy, err := NewMetadataPolicy(MetadataPolicyOptions{MaxInputEntries: 1, MaxEntries: 1})
	if err != nil {
		t.Fatalf("NewMetadataPolicy() error: %v", err)
	}
	input := map[string]string{"first": "value", "second": "value"}
	if _, _, err := policy.Filter(input); !errors.Is(err, ErrMetadataLimit) {
		t.Fatalf("Filter() error = %v, want ErrMetadataLimit", err)
	}
}

func TestMetadataPolicyRejectsUnsafeUnapprovedKey(t *testing.T) {
	t.Parallel()

	policy, err := NewMetadataPolicy(MetadataPolicyOptions{})
	if err != nil {
		t.Fatalf("NewMetadataPolicy() error: %v", err)
	}
	if _, _, err := policy.Filter(map[string]string{"user.prompt": "secret"}); !errors.Is(err, ErrSensitiveField) {
		t.Fatalf("Filter() error = %v, want ErrSensitiveField", err)
	}
}

func TestMetadataPolicyRejectsInvalidAllowedValue(t *testing.T) {
	t.Parallel()

	policy, err := NewMetadataPolicy(MetadataPolicyOptions{AllowedKeys: []string{"region"}, MaxValueBytes: 4})
	if err != nil {
		t.Fatalf("NewMetadataPolicy() error: %v", err)
	}
	if _, _, err := policy.Filter(map[string]string{"region": "too-long"}); !errors.Is(err, ErrMetadataLimit) {
		t.Fatalf("Filter() error = %v, want ErrMetadataLimit", err)
	}
}
