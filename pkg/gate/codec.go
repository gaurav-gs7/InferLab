package gate

import "io"

func DecodeEvaluation(reader io.Reader) (Evaluation, error) {
	evaluation, err := decodeStrict[Evaluation](reader, MaxDocumentBytes, ErrInvalidEvaluation)
	if err != nil {
		return Evaluation{}, err
	}
	if err := ValidateEvaluation(evaluation); err != nil {
		return Evaluation{}, err
	}
	return evaluation, nil
}

func DecodeResult(reader io.Reader) (Result, error) {
	result, err := decodeStrict[Result](reader, MaxDocumentBytes, ErrInvalidResult)
	if err != nil {
		return Result{}, err
	}
	if err := ValidateResult(result); err != nil {
		return Result{}, err
	}
	return result, nil
}
