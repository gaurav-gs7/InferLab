package change

import "errors"

var (
	// ErrInvalidDocument indicates malformed or internally inconsistent input.
	ErrInvalidDocument = errors.New("invalid inference change")
	// ErrUnsupportedSchema indicates a document for a different schema.
	ErrUnsupportedSchema = errors.New("unsupported inference-change schema")
	// ErrUnsupportedVersion indicates an unsupported schema version.
	ErrUnsupportedVersion = errors.New("unsupported inference-change version")
	// ErrUnsupportedFeature indicates valid syntax outside the v0.1 support envelope.
	ErrUnsupportedFeature = errors.New("unsupported inference-change feature")
	// ErrDocumentTooLarge indicates that the bounded decoder rejected the input.
	ErrDocumentTooLarge = errors.New("inference-change document exceeds size limit")
)
