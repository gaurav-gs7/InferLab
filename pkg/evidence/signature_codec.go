package evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// DecodeRuntimeSignature reads one bounded strict JSON runtime signature.
func DecodeRuntimeSignature(reader io.Reader) (RuntimeSignature, error) {
	if reader == nil {
		return RuntimeSignature{}, fmt.Errorf("%w: reader is nil", ErrInvalidSignature)
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxDocumentBytes+1))
	if err != nil {
		return RuntimeSignature{}, fmt.Errorf("read runtime signature: %w", err)
	}
	if len(data) > MaxDocumentBytes {
		return RuntimeSignature{}, ErrDocumentTooLarge
	}
	if err := validateJSONShape(data, ErrInvalidSignature); err != nil {
		return RuntimeSignature{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var signature RuntimeSignature
	if err := decoder.Decode(&signature); err != nil {
		return RuntimeSignature{}, fmt.Errorf("%w: decode JSON: %v", ErrInvalidSignature, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return RuntimeSignature{}, fmt.Errorf("%w: trailing JSON value", ErrInvalidSignature)
		}
		return RuntimeSignature{}, fmt.Errorf("%w: trailing content: %v", ErrInvalidSignature, err)
	}
	if err := ValidateRuntimeSignature(signature); err != nil {
		return RuntimeSignature{}, err
	}
	return signature, nil
}
