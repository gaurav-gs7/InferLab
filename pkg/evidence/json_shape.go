package evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const maxJSONDepth = 64

func validateJSONShape(data []byte, sentinel error) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := walkJSON(decoder, 0, sentinel); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return fmt.Errorf("%w: multiple JSON values", sentinel)
		}
		return fmt.Errorf("%w: JSON: %v", sentinel, err)
	}
	return nil
}

func walkJSON(decoder *json.Decoder, depth int, sentinel error) error {
	if depth > maxJSONDepth {
		return fmt.Errorf("%w: maximum depth is %d", ErrNestingLimit, maxJSONDepth)
	}
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("%w: JSON: %v", sentinel, err)
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
				return fmt.Errorf("%w: JSON object key: %v", sentinel, err)
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("%w: JSON object key is not a string", sentinel)
			}
			if _, exists := seen[key]; exists {
				return fmt.Errorf("%w: %q", ErrDuplicateField, key)
			}
			seen[key] = struct{}{}
			if err := walkJSON(decoder, depth+1, sentinel); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return fmt.Errorf("%w: malformed JSON object", sentinel)
		}
	case '[':
		for decoder.More() {
			if err := walkJSON(decoder, depth+1, sentinel); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return fmt.Errorf("%w: malformed JSON array", sentinel)
		}
	default:
		return fmt.Errorf("%w: unexpected JSON delimiter %q", sentinel, delimiter)
	}
	return nil
}
