package adapter

import (
	"fmt"
	"io"
)

func DecodeInput(reader io.Reader) (Input, error) {
	input, err := decodeStrict[Input](reader, MaxInputBytes, ErrInvalidInput)
	if err != nil {
		return Input{}, err
	}
	if err := ValidateInput(input); err != nil {
		return Input{}, err
	}
	return input, nil
}

func DecodeNormalizedReport(reader io.Reader) (NormalizedReport, error) {
	report, err := decodeStrict[NormalizedReport](reader, DefaultMaxOutputBytes, ErrInvalidReport)
	if err != nil {
		return NormalizedReport{}, err
	}
	if err := ValidateNormalizedReport(report); err != nil {
		return NormalizedReport{}, err
	}
	return report, nil
}

func DecodeRequest(reader io.Reader) (Request, error) {
	request, err := decodeStrict[Request](reader, MaxInputBytes, ErrProtocol)
	if err != nil {
		return Request{}, err
	}
	if err := ValidateRequest(request); err != nil {
		return Request{}, err
	}
	return request, nil
}

func DecodeResponse(reader io.Reader, maxBytes int64) (Response, error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxOutputBytes
	}
	response, err := decodeStrict[Response](reader, maxBytes, ErrProtocol)
	if err != nil {
		return Response{}, err
	}
	if err := ValidateResponse(response); err != nil {
		return Response{}, err
	}
	return response, nil
}

func ValidateRequest(request Request) error {
	if request.Schema != ProtocolSchema || request.SchemaVersion != CurrentVersion || !namePattern.MatchString(request.RequestID) {
		return fmt.Errorf("%w: invalid request identity", ErrProtocol)
	}
	switch request.Operation {
	case OperationCapabilities:
		if request.Input != nil {
			return fmt.Errorf("%w: capabilities request must not contain input", ErrProtocol)
		}
	case OperationNormalize:
		if request.Input == nil {
			return fmt.Errorf("%w: normalize request requires input", ErrProtocol)
		}
		if err := ValidateInput(*request.Input); err != nil {
			return fmt.Errorf("%w: %w", ErrProtocol, err)
		}
	default:
		return fmt.Errorf("%w: unsupported operation %q", ErrProtocol, request.Operation)
	}
	return nil
}

func ValidateResponse(response Response) error {
	if response.Schema != ProtocolSchema || response.SchemaVersion != CurrentVersion || !namePattern.MatchString(response.RequestID) {
		return fmt.Errorf("%w: invalid response identity", ErrProtocol)
	}
	count := 0
	if response.Capabilities != nil {
		count++
		if err := ValidateCapabilities(*response.Capabilities); err != nil {
			return fmt.Errorf("%w: %w", ErrProtocol, err)
		}
	}
	if response.Report != nil {
		count++
		if err := ValidateNormalizedReport(*response.Report); err != nil {
			return fmt.Errorf("%w: %w", ErrProtocol, err)
		}
	}
	if response.Failure != nil {
		count++
		if !namePattern.MatchString(response.Failure.Code) || response.Failure.Message == "" || len(response.Failure.Message) > 1024 {
			return fmt.Errorf("%w: invalid failure", ErrProtocol)
		}
	}
	if count != 1 {
		return fmt.Errorf("%w: response requires exactly one payload", ErrProtocol)
	}
	return nil
}
