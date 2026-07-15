package gate

import "errors"

var (
	ErrInvalidEvaluation           = errors.New("invalid gate evaluation")
	ErrInvalidResult               = errors.New("invalid gate result")
	ErrUnsupportedMetric           = errors.New("unsupported gate metric")
	ErrUnsupportedFault            = errors.New("unsupported fault evidence")
	ErrInvalidCounterexample       = errors.New("invalid counterexample")
	ErrCounterexampleDoesNotFail   = errors.New("counterexample does not reproduce the failure")
	ErrCounterexampleNotReproduced = errors.New("minimized counterexample did not pass re-verification")
	ErrMinimizationBudget          = errors.New("counterexample minimization budget is insufficient")
)
