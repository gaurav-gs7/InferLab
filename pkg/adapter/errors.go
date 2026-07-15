package adapter

import "errors"

var (
	ErrInvalidCapabilities = errors.New("invalid adapter capabilities")
	ErrInvalidInput        = errors.New("invalid adapter input")
	ErrInvalidReport       = errors.New("invalid normalized report")
	ErrUnsupportedProducer = errors.New("unsupported evidence producer")
	ErrUnsupportedMetric   = errors.New("unsupported producer metric")
	ErrClassification      = errors.New("evidence classification violation")
	ErrProtocol            = errors.New("adapter protocol violation")
	ErrOutputLimit         = errors.New("adapter output exceeds limit")
	ErrAdapterFailed       = errors.New("adapter process failed")
)
