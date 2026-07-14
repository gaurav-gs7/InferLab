package trace

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestValidateRecord(t *testing.T) {
	t.Parallel()

	if err := Validate(validRecord()); err != nil {
		t.Fatalf("Validate() error: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Record)
		want   error
	}{
		{name: "schema", mutate: func(r *Record) { r.Schema = "other" }, want: ErrUnsupportedSchema},
		{name: "version format", mutate: func(r *Record) { r.SchemaVersion = "01.0" }, want: ErrUnsupportedVersion},
		{name: "version minor", mutate: func(r *Record) { r.SchemaVersion = "1.1" }, want: ErrUnsupportedVersion},
		{name: "sequence", mutate: func(r *Record) { r.Sequence = 0 }, want: ErrInvalidRecord},
		{name: "arrival", mutate: func(r *Record) { r.ArrivalOffsetNS = -1 }, want: ErrInvalidRecord},
		{name: "request ID", mutate: func(r *Record) { r.RequestID = "" }, want: ErrInvalidRecord},
		{name: "tenant pseudonym", mutate: func(r *Record) { r.TenantID = "payments" }, want: ErrInvalidRecord},
		{name: "model", mutate: func(r *Record) { r.Model = "model\nname" }, want: ErrInvalidRecord},
		{name: "input tokens", mutate: func(r *Record) { r.InputTokens = 0 }, want: ErrTokenLimit},
		{name: "maximum output tokens", mutate: func(r *Record) { r.MaxOutputTokens = 0 }, want: ErrTokenLimit},
		{name: "observed output tokens", mutate: func(r *Record) { r.OutputTokens = r.MaxOutputTokens + 1 }, want: ErrTokenLimit},
		{name: "priority", mutate: func(r *Record) { r.Priority = 101 }, want: ErrInvalidRecord},
		{name: "prefix fingerprint", mutate: func(r *Record) { r.PrefixFingerprint = "sha256:raw" }, want: ErrInvalidRecord},
		{name: "adapter", mutate: func(r *Record) { r.Adapter = strings.Repeat("a", maxIdentifierBytes+1) }, want: ErrInvalidRecord},
		{name: "selected endpoint", mutate: func(r *Record) { r.SelectedEndpoint = "worker\n7" }, want: ErrInvalidRecord},
		{name: "TTFT", mutate: func(r *Record) { value := math.NaN(); r.ObservedTTFTMS = &value }, want: ErrInvalidRecord},
		{name: "TPOT", mutate: func(r *Record) { value := math.Inf(1); r.ObservedTPOTMS = &value }, want: ErrInvalidRecord},
		{name: "metadata key", mutate: func(r *Record) { r.Metadata = map[string]string{"user.prompt": "redacted"} }, want: ErrSensitiveField},
		{name: "metadata value", mutate: func(r *Record) { r.Metadata = map[string]string{"region": "bad\nvalue"} }, want: ErrInvalidRecord},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			record := validRecord()
			tt.mutate(&record)
			if err := Validate(record); !errors.Is(err, tt.want) {
				t.Fatalf("Validate() error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestValidateCustomLimitsAvoidsTokenUnderflow(t *testing.T) {
	t.Parallel()

	record := validRecord()
	limits := DefaultLimits()
	limits.MaxTotalTokens = 100
	if err := validateRecord(record, limits, false); !errors.Is(err, ErrTokenLimit) {
		t.Fatalf("validateRecord() error = %v, want ErrTokenLimit", err)
	}
}

func TestNewRecord(t *testing.T) {
	t.Parallel()

	record := NewRecord()
	if record.Schema != Schema || record.SchemaVersion != CurrentSchemaVersion {
		t.Fatalf("NewRecord() = %#v", record)
	}
}

func TestNilHelpersFailWithoutPanicking(t *testing.T) {
	t.Parallel()

	if _, err := NewDecoder(nil, Limits{}).Decode(); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("nil decoder error = %v, want ErrInvalidRecord", err)
	}
	if err := NewEncoder(nil, Limits{}).Encode(validRecord()); !errors.Is(err, ErrEncoderFailed) {
		t.Fatalf("nil encoder error = %v, want ErrEncoderFailed", err)
	}
	var protector *Protector
	if _, err := protector.TenantID("tenant"); !errors.Is(err, ErrInvalidProtectionKey) {
		t.Fatalf("nil protector error = %v, want ErrInvalidProtectionKey", err)
	}
	var policy *MetadataPolicy
	if _, _, err := policy.Filter(nil); !errors.Is(err, ErrInvalidRecord) {
		t.Fatalf("nil policy error = %v, want ErrInvalidRecord", err)
	}
}
