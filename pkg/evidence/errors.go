package evidence

import "errors"

var (
	ErrInvalidSignature   = errors.New("invalid runtime signature")
	ErrUnsupportedSchema  = errors.New("unsupported evidence schema")
	ErrUnsupportedVersion = errors.New("unsupported evidence schema version")
	ErrInvalidEnvelope    = errors.New("invalid evidence envelope")
	ErrIncompleteEvidence = errors.New("incomplete evidence")
	ErrDocumentTooLarge   = errors.New("evidence document exceeds size limit")
	ErrInvalidPolicy      = errors.New("invalid compatibility policy")
	ErrInvalidGraph       = errors.New("invalid evidence validity graph")
	ErrDuplicateField     = errors.New("duplicate evidence JSON field")
	ErrNestingLimit       = errors.New("evidence JSON nesting limit exceeded")
)
