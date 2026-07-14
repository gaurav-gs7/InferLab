package change

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Decode reads one bounded JSON document, rejects unknown fields and trailing
// values, and validates the v0.1 support envelope.
func Decode(reader io.Reader) (Document, error) {
	if reader == nil {
		return Document{}, fmt.Errorf("%w: reader is nil", ErrInvalidDocument)
	}
	data, err := io.ReadAll(io.LimitReader(reader, MaxDocumentBytes+1))
	if err != nil {
		return Document{}, fmt.Errorf("read inference change: %w", err)
	}
	if len(data) > MaxDocumentBytes {
		return Document{}, ErrDocumentTooLarge
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var document Document
	if err := decoder.Decode(&document); err != nil {
		return Document{}, fmt.Errorf("%w: decode JSON: %v", ErrInvalidDocument, err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return Document{}, fmt.Errorf("%w: trailing JSON value", ErrInvalidDocument)
		}
		return Document{}, fmt.Errorf("%w: trailing content: %v", ErrInvalidDocument, err)
	}
	if err := Validate(document); err != nil {
		return Document{}, err
	}
	return document, nil
}

// CanonicalJSON returns the stable JSON representation used for evidence IDs.
func CanonicalJSON(document Document) ([]byte, error) {
	if err := Validate(document); err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(canonicalCopy(document))
	if err != nil {
		return nil, fmt.Errorf("encode canonical inference change: %w", err)
	}
	return encoded, nil
}

// Digest returns the sha256-prefixed digest of CanonicalJSON.
func Digest(document Document) (string, error) {
	canonical, err := CanonicalJSON(document)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
