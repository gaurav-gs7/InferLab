package trace

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidRecord        = errors.New("invalid trace record")
	ErrUnsupportedSchema    = errors.New("unsupported trace schema")
	ErrUnsupportedVersion   = errors.New("unsupported trace schema version")
	ErrSensitiveField       = errors.New("sensitive content field is forbidden")
	ErrDuplicateField       = errors.New("duplicate JSON field")
	ErrRecordTooLarge       = errors.New("trace record exceeds byte limit")
	ErrTraceTooLarge        = errors.New("trace exceeds byte limit")
	ErrRecordLimit          = errors.New("trace exceeds record limit")
	ErrTokenLimit           = errors.New("trace record exceeds token limit")
	ErrMetadataLimit        = errors.New("trace metadata exceeds limit")
	ErrEncoderFailed        = errors.New("trace encoder is in a failed state")
	ErrInvalidProtectionKey = errors.New("invalid trace protection key")
)

// DecodeError identifies the record and byte offset that failed. Record is
// one-based and ByteOffset is zero-based from the start of the stream.
type DecodeError struct {
	Record     uint64
	ByteOffset int64
	Err        error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("decode trace record %d at byte %d: %v", e.Record, e.ByteOffset, e.Err)
}

func (e *DecodeError) Unwrap() error {
	return e.Err
}
