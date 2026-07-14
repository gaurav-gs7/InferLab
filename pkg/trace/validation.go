package trace

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxRequestIDBytes  = 128
	maxIdentifierBytes = 256
	tenantDigestPrefix = "tenant-hmac-sha256:"
	prefixDigestPrefix = "prefix-hmac-sha256:"
)

// Validate checks a record against the current schema and default limits.
// It is intended for records that will be emitted by this package.
func Validate(record Record) error {
	return validateRecord(record, DefaultLimits(), false)
}

func validateRecord(record Record, limits Limits, allowFutureMinor bool) error {
	limits = normalizeLimits(limits)
	if record.Schema != Schema {
		return fmt.Errorf("%w: %q", ErrUnsupportedSchema, record.Schema)
	}
	major, minor, err := parseVersion(record.SchemaVersion)
	if err != nil || major != SupportedSchemaMajor {
		return fmt.Errorf("%w: %q", ErrUnsupportedVersion, record.SchemaVersion)
	}
	_, currentMinor, _ := parseVersion(CurrentSchemaVersion)
	if !allowFutureMinor && minor != currentMinor {
		return fmt.Errorf("%w: cannot emit %q", ErrUnsupportedVersion, record.SchemaVersion)
	}
	if record.Sequence == 0 {
		return fmt.Errorf("%w: sequence must be greater than zero", ErrInvalidRecord)
	}
	if record.ArrivalOffsetNS < 0 {
		return fmt.Errorf("%w: arrival offset cannot be negative", ErrInvalidRecord)
	}
	if err := validateIdentifier("request ID", record.RequestID, maxRequestIDBytes); err != nil {
		return err
	}
	if !validDigest(record.TenantID, tenantDigestPrefix) {
		return fmt.Errorf("%w: tenant ID must be an HMAC-SHA256 pseudonym", ErrInvalidRecord)
	}
	if err := validateIdentifier("model", record.Model, maxIdentifierBytes); err != nil {
		return err
	}
	if record.InputTokens == 0 || record.InputTokens > limits.MaxInputTokens {
		return fmt.Errorf("%w: input tokens %d exceed range 1..%d", ErrTokenLimit, record.InputTokens, limits.MaxInputTokens)
	}
	if record.MaxOutputTokens == 0 || record.MaxOutputTokens > limits.MaxOutputTokens {
		return fmt.Errorf("%w: maximum output tokens %d exceed range 1..%d", ErrTokenLimit, record.MaxOutputTokens, limits.MaxOutputTokens)
	}
	if record.OutputTokens > record.MaxOutputTokens || record.OutputTokens > limits.MaxOutputTokens {
		return fmt.Errorf("%w: observed output tokens %d exceed maximum", ErrTokenLimit, record.OutputTokens)
	}
	if record.MaxOutputTokens > limits.MaxTotalTokens || record.InputTokens > limits.MaxTotalTokens-record.MaxOutputTokens {
		return fmt.Errorf("%w: input plus maximum output tokens exceed %d", ErrTokenLimit, limits.MaxTotalTokens)
	}
	if record.Priority > 100 {
		return fmt.Errorf("%w: priority must be between 0 and 100", ErrInvalidRecord)
	}
	if record.PrefixFingerprint != "" && !validDigest(record.PrefixFingerprint, prefixDigestPrefix) {
		return fmt.Errorf("%w: prefix fingerprint must be a keyed HMAC-SHA256 digest", ErrInvalidRecord)
	}
	if record.Adapter != "" {
		if err := validateIdentifier("adapter", record.Adapter, maxIdentifierBytes); err != nil {
			return err
		}
	}
	if record.SelectedEndpoint != "" {
		if err := validateIdentifier("selected endpoint", record.SelectedEndpoint, maxIdentifierBytes); err != nil {
			return err
		}
	}
	if err := validateOptionalMetric("observed TTFT", record.ObservedTTFTMS); err != nil {
		return err
	}
	if err := validateOptionalMetric("observed TPOT", record.ObservedTPOTMS); err != nil {
		return err
	}
	if len(record.Metadata) > limits.MaxMetadataEntries {
		return fmt.Errorf("%w: got %d entries, maximum is %d", ErrMetadataLimit, len(record.Metadata), limits.MaxMetadataEntries)
	}
	metadataKeys := make([]string, 0, len(record.Metadata))
	for key := range record.Metadata {
		metadataKeys = append(metadataKeys, key)
	}
	sort.Strings(metadataKeys)
	for _, key := range metadataKeys {
		if err := validateMetadataKey(key, limits.MaxMetadataKeyBytes); err != nil {
			return err
		}
		if err := validateMetadataValue(key, record.Metadata[key], limits.MaxMetadataValueBytes); err != nil {
			return err
		}
	}
	return nil
}

func parseVersion(version string) (int, int, error) {
	parts := strings.Split(version, ".")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return 0, 0, errors.New("schema version must be MAJOR.MINOR")
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil || major < 0 {
		return 0, 0, errors.New("invalid schema major version")
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil || minor < 0 {
		return 0, 0, errors.New("invalid schema minor version")
	}
	if strconv.Itoa(major)+"."+strconv.Itoa(minor) != version {
		return 0, 0, errors.New("schema version is not canonical")
	}
	return major, minor, nil
}

func validDigest(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) {
		return false
	}
	digest := strings.TrimPrefix(value, prefix)
	if len(digest) != 64 {
		return false
	}
	_, err := hex.DecodeString(digest)
	return err == nil && digest == strings.ToLower(digest)
}

func validateIdentifier(name, value string, maxBytes int) error {
	if value == "" {
		return fmt.Errorf("%w: %s is required", ErrInvalidRecord, name)
	}
	if len(value) > maxBytes || !utf8.ValidString(value) {
		return fmt.Errorf("%w: %s must be valid UTF-8 and at most %d bytes", ErrInvalidRecord, name, maxBytes)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: %s contains control characters", ErrInvalidRecord, name)
		}
	}
	return nil
}

func validateOptionalMetric(name string, value *float64) error {
	if value == nil {
		return nil
	}
	if *value < 0 || math.IsNaN(*value) || math.IsInf(*value, 0) {
		return fmt.Errorf("%w: %s must be finite and non-negative", ErrInvalidRecord, name)
	}
	return nil
}
