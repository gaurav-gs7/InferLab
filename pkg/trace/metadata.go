package trace

import (
	"fmt"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const defaultMaxMetadataInputEntries = 256

// MetadataPolicyOptions configures explicit metadata capture. An empty
// allowlist captures no optional metadata.
type MetadataPolicyOptions struct {
	AllowedKeys     []string
	MaxInputEntries int
	MaxEntries      int
	MaxValueBytes   int
}

// MetadataPolicy copies only explicitly allowed, non-sensitive metadata keys.
// It is immutable after construction and safe for concurrent use.
type MetadataPolicy struct {
	allowed         map[string]struct{}
	maxInputEntries int
	maxEntries      int
	maxValueBytes   int
}

// NewMetadataPolicy validates an allowlist. Keys that could carry request or
// response content are rejected even when explicitly configured.
func NewMetadataPolicy(options MetadataPolicyOptions) (*MetadataPolicy, error) {
	defaults := DefaultLimits()
	if options.MaxEntries <= 0 {
		options.MaxEntries = defaults.MaxMetadataEntries
	}
	if options.MaxInputEntries <= 0 {
		options.MaxInputEntries = defaultMaxMetadataInputEntries
	}
	if options.MaxInputEntries < options.MaxEntries {
		return nil, fmt.Errorf("%w: input-entry limit %d is below output-entry limit %d", ErrMetadataLimit, options.MaxInputEntries, options.MaxEntries)
	}
	if options.MaxValueBytes <= 0 {
		options.MaxValueBytes = defaults.MaxMetadataValueBytes
	}
	if len(options.AllowedKeys) > options.MaxEntries {
		return nil, fmt.Errorf("%w: allowlist has %d keys, maximum is %d", ErrMetadataLimit, len(options.AllowedKeys), options.MaxEntries)
	}
	allowed := make(map[string]struct{}, len(options.AllowedKeys))
	for _, key := range options.AllowedKeys {
		if err := validateMetadataKey(key, defaults.MaxMetadataKeyBytes); err != nil {
			return nil, err
		}
		if _, exists := allowed[key]; exists {
			return nil, fmt.Errorf("%w: duplicate allowlist key %q", ErrInvalidRecord, key)
		}
		allowed[key] = struct{}{}
	}
	return &MetadataPolicy{
		allowed:         allowed,
		maxInputEntries: options.MaxInputEntries,
		maxEntries:      options.MaxEntries,
		maxValueBytes:   options.MaxValueBytes,
	}, nil
}

// Filter returns a fresh map containing only allowed keys and a sorted list of
// keys that were dropped. Invalid values for allowed keys fail closed.
func (p *MetadataPolicy) Filter(input map[string]string) (map[string]string, []string, error) {
	if p == nil {
		return nil, nil, fmt.Errorf("%w: metadata policy is nil", ErrInvalidRecord)
	}
	if len(input) > p.maxInputEntries {
		return nil, nil, fmt.Errorf("%w: input has %d entries, maximum scanned is %d", ErrMetadataLimit, len(input), p.maxInputEntries)
	}
	keys := make([]string, 0, len(input))
	for key := range input {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	filtered := make(map[string]string, min(len(input), p.maxEntries))
	dropped := make([]string, 0)
	for _, key := range keys {
		if err := validateMetadataKey(key, DefaultLimits().MaxMetadataKeyBytes); err != nil {
			return nil, nil, err
		}
		if _, allowed := p.allowed[key]; !allowed {
			dropped = append(dropped, key)
			continue
		}
		if len(filtered) == p.maxEntries {
			return nil, dropped, fmt.Errorf("%w: filtered metadata exceeds %d entries", ErrMetadataLimit, p.maxEntries)
		}
		if err := validateMetadataValue(key, input[key], p.maxValueBytes); err != nil {
			return nil, dropped, err
		}
		filtered[key] = input[key]
	}
	if len(filtered) == 0 {
		filtered = nil
	}
	return filtered, dropped, nil
}

func validateMetadataKey(key string, maxBytes int) error {
	if key == "" || len(key) > maxBytes || !utf8.ValidString(key) {
		return fmt.Errorf("%w: metadata key must be valid UTF-8 and contain 1..%d bytes", ErrInvalidRecord, maxBytes)
	}
	for i, r := range key {
		valid := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '.' || r == '_' || r == '-'
		if !valid || i == 0 && r >= '0' && r <= '9' {
			return fmt.Errorf("%w: metadata key %q must use lowercase letters, digits, '.', '_' or '-' and start with a letter", ErrInvalidRecord, key)
		}
	}
	if sensitiveFieldName(key) {
		return fmt.Errorf("%w: metadata key %q", ErrSensitiveField, key)
	}
	return nil
}

func validateMetadataValue(key, value string, maxBytes int) error {
	if len(value) > maxBytes || !utf8.ValidString(value) {
		return fmt.Errorf("%w: metadata value for %q must be valid UTF-8 and at most %d bytes", ErrMetadataLimit, key, maxBytes)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: metadata value for %q contains control characters", ErrInvalidRecord, key)
		}
	}
	return nil
}

func sensitiveFieldName(name string) bool {
	normalized := strings.NewReplacer("-", "_", ".", "_").Replace(strings.ToLower(name))
	if normalized == "input" || normalized == "output" || normalized == "body" || normalized == "request_body" || normalized == "response_body" {
		return true
	}
	for _, fragment := range []string{"prompt", "content", "message", "completion", "generated_text", "raw_request", "raw_response"} {
		if strings.Contains(normalized, fragment) {
			return true
		}
	}
	return false
}
