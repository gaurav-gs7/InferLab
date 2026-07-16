package safetycase

import "errors"

var (
	ErrInvalidDescriptor = errors.New("invalid safety-case descriptor")
	ErrInvalidManifest   = errors.New("invalid safety-case manifest")
	ErrInvalidSignature  = errors.New("invalid safety-case signature")
	ErrUnsafePath        = errors.New("unsafe artifact path")
	ErrArtifactMismatch  = errors.New("artifact closure mismatch")
	ErrLinkageMismatch   = errors.New("gate linkage mismatch")
)
