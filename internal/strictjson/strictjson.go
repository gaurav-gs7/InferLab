// Package strictjson provides bounded, single-value JSON decoding primitives
// for security-sensitive public document contracts.
package strictjson

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const MaxDepth = 64

var (
	ErrDuplicateField = errors.New("duplicate JSON field")
	ErrMultipleValues = errors.New("multiple JSON values")
	ErrNestingLimit   = errors.New("JSON nesting limit exceeded")
	ErrTooLarge       = errors.New("JSON document exceeds size limit")
)

// ReadOne reads one JSON value within maxBytes and rejects duplicate object
// fields, excessive nesting, malformed input, and trailing values.
func ReadOne(reader io.Reader, maxBytes int64) ([]byte, error) {
	if reader == nil {
		return nil, errors.New("reader is nil")
	}
	if maxBytes <= 0 {
		return nil, errors.New("maximum document size must be positive")
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read JSON: %w", err)
	}
	if int64(len(data)) > maxBytes {
		return nil, ErrTooLarge
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := walk(decoder, 0); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return nil, ErrMultipleValues
		}
		return nil, fmt.Errorf("trailing JSON: %w", err)
	}
	return data, nil
}

func walk(decoder *json.Decoder, depth int) error {
	if depth > MaxDepth {
		return fmt.Errorf("%w: maximum depth is %d", ErrNestingLimit, MaxDepth)
	}
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("decode JSON token: %w", err)
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("decode JSON object key: %w", err)
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("JSON object key is not a string")
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%w: %q", ErrDuplicateField, key)
			}
			seen[key] = struct{}{}
			if err := walk(decoder, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return errors.New("malformed JSON object")
		}
	case '[':
		for decoder.More() {
			if err := walk(decoder, depth+1); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return errors.New("malformed JSON array")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delimiter)
	}
	return nil
}
