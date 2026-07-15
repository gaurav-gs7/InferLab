package gate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
)

const (
	CounterexampleSchema      = "inferlab.counterexample"
	maxCounterexampleRequests = 10000
)

type Counterexample struct {
	Schema        string                  `json:"schema"`
	SchemaVersion string                  `json:"schema_version"`
	Name          string                  `json:"name"`
	Concurrency   uint32                  `json:"concurrency"`
	Requests      []CounterexampleRequest `json:"requests"`
	Fault         FaultPoint              `json:"fault"`
}

type CounterexampleRequest struct {
	ID                        string `json:"id"`
	ArrivalOffsetMilliseconds uint64 `json:"arrival_offset_milliseconds"`
	PromptTokens              uint64 `json:"prompt_tokens"`
	OutputTokens              uint64 `json:"output_tokens"`
	Tenant                    string `json:"tenant"`
}

type Oracle func(context.Context, Counterexample) (bool, error)

type MinimizeOptions struct {
	MaximumEvaluations uint32 `json:"maximum_evaluations"`
	VerificationRuns   uint32 `json:"verification_runs"`
}

type MinimizeStep struct {
	Operation    string `json:"operation"`
	BeforeDigest string `json:"before_digest"`
	AfterDigest  string `json:"after_digest"`
}

type MinimizeResult struct {
	InitialDigest        string         `json:"initial_digest"`
	CounterexampleDigest string         `json:"counterexample_digest"`
	Counterexample       Counterexample `json:"counterexample"`
	Evaluations          uint32         `json:"evaluations"`
	VerificationRuns     uint32         `json:"verification_runs"`
	SearchComplete       bool           `json:"search_complete"`
	Steps                []MinimizeStep `json:"steps"`
}

func NewCounterexample() Counterexample {
	return Counterexample{Schema: CounterexampleSchema, SchemaVersion: CurrentSchemaVersion}
}

func ValidateCounterexample(counterexample Counterexample) error {
	if counterexample.Schema != CounterexampleSchema || counterexample.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("%w: unsupported schema or version", ErrInvalidCounterexample)
	}
	if !namePattern.MatchString(counterexample.Name) || counterexample.Concurrency == 0 || counterexample.Concurrency > 4096 {
		return fmt.Errorf("%w: name or concurrency is invalid", ErrInvalidCounterexample)
	}
	if len(counterexample.Requests) == 0 || len(counterexample.Requests) > maxCounterexampleRequests {
		return fmt.Errorf("%w: requests must contain 1..%d entries", ErrInvalidCounterexample, maxCounterexampleRequests)
	}
	seen := make(map[string]struct{}, len(counterexample.Requests))
	var previousArrival uint64
	for i, request := range counterexample.Requests {
		if !namePattern.MatchString(request.ID) || !namePattern.MatchString(request.Tenant) || request.PromptTokens == 0 || request.PromptTokens > 1048576 || request.OutputTokens == 0 || request.OutputTokens > 1048576 || request.ArrivalOffsetMilliseconds > 24*60*60*1000 {
			return fmt.Errorf("%w: requests[%d] is invalid", ErrInvalidCounterexample, i)
		}
		if i > 0 && request.ArrivalOffsetMilliseconds < previousArrival {
			return fmt.Errorf("%w: request arrivals must be non-decreasing", ErrInvalidCounterexample)
		}
		previousArrival = request.ArrivalOffsetMilliseconds
		if _, exists := seen[request.ID]; exists {
			return fmt.Errorf("%w: duplicate request id %q", ErrInvalidCounterexample, request.ID)
		}
		seen[request.ID] = struct{}{}
	}
	if err := ValidateFaultPoint(counterexample.Fault); err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidCounterexample, err)
	}
	return nil
}

func CounterexampleCanonicalJSON(counterexample Counterexample) ([]byte, error) {
	if err := ValidateCounterexample(counterexample); err != nil {
		return nil, err
	}
	return json.Marshal(counterexample)
}

func CounterexampleDigest(counterexample Counterexample) (string, error) {
	canonical, err := CounterexampleCanonicalJSON(counterexample)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

// Minimize performs deterministic delta debugging and bounded numeric
// reductions, then requires the minimized candidate to reproduce repeatedly.
// SearchComplete reports whether every declared transformation was exhausted;
// a budget-limited but reverified result is still explicitly marked partial.
func Minimize(ctx context.Context, initial Counterexample, options MinimizeOptions, oracle Oracle) (MinimizeResult, error) {
	if ctx == nil || oracle == nil {
		return MinimizeResult{}, fmt.Errorf("%w: context and oracle are required", ErrInvalidCounterexample)
	}
	if err := ValidateCounterexample(initial); err != nil {
		return MinimizeResult{}, err
	}
	if options.MaximumEvaluations < 2 || options.MaximumEvaluations > 100000 || options.VerificationRuns == 0 || options.VerificationRuns > 20 || options.MaximumEvaluations <= options.VerificationRuns {
		return MinimizeResult{}, ErrMinimizationBudget
	}
	initialDigest, _ := CounterexampleDigest(initial)
	state := minimizer{
		ctx: ctx, oracle: oracle,
		searchLimit: options.MaximumEvaluations - options.VerificationRuns,
		candidate:   cloneCounterexample(initial),
	}
	fails, err := state.evaluate(state.candidate)
	if err != nil {
		return MinimizeResult{}, err
	}
	if !fails {
		return MinimizeResult{}, ErrCounterexampleDoesNotFail
	}
	state.minimizeRequests()
	state.minimizeConcurrency()
	state.minimizeFault()
	state.minimizeRequestShapes()
	if state.err != nil {
		return MinimizeResult{}, state.err
	}

	for i := uint32(0); i < options.VerificationRuns; i++ {
		if err := ctx.Err(); err != nil {
			return MinimizeResult{}, err
		}
		fails, oracleErr := oracle(ctx, cloneCounterexample(state.candidate))
		state.evaluations++
		if oracleErr != nil {
			return MinimizeResult{}, fmt.Errorf("counterexample verification: %w", oracleErr)
		}
		if !fails {
			return MinimizeResult{}, ErrCounterexampleNotReproduced
		}
	}
	digest, err := CounterexampleDigest(state.candidate)
	if err != nil {
		return MinimizeResult{}, err
	}
	return MinimizeResult{
		InitialDigest: initialDigest, CounterexampleDigest: digest, Counterexample: state.candidate,
		Evaluations: state.evaluations, VerificationRuns: options.VerificationRuns,
		SearchComplete: !state.exhausted, Steps: state.steps,
	}, nil
}

type minimizer struct {
	ctx         context.Context
	oracle      Oracle
	searchLimit uint32
	evaluations uint32
	exhausted   bool
	err         error
	candidate   Counterexample
	steps       []MinimizeStep
}

func (state *minimizer) evaluate(candidate Counterexample) (bool, error) {
	if state.evaluations >= state.searchLimit {
		state.exhausted = true
		return false, nil
	}
	if err := state.ctx.Err(); err != nil {
		return false, err
	}
	if err := ValidateCounterexample(candidate); err != nil {
		return false, nil
	}
	fails, err := state.oracle(state.ctx, cloneCounterexample(candidate))
	state.evaluations++
	if err != nil {
		return false, fmt.Errorf("counterexample oracle: %w", err)
	}
	return fails, nil
}

func (state *minimizer) accept(operation string, candidate Counterexample) {
	before, _ := CounterexampleDigest(state.candidate)
	after, _ := CounterexampleDigest(candidate)
	state.steps = append(state.steps, MinimizeStep{Operation: operation, BeforeDigest: before, AfterDigest: after})
	state.candidate = candidate
}

func (state *minimizer) try(operation string, candidate Counterexample) bool {
	if state.exhausted {
		return false
	}
	fails, err := state.evaluate(candidate)
	if err != nil {
		state.exhausted = true
		state.err = err
		return false
	}
	if fails {
		state.accept(operation, candidate)
		return true
	}
	return false
}

func (state *minimizer) minimizeRequests() {
	granularity := 2
	for len(state.candidate.Requests) >= 2 && !state.exhausted {
		length := len(state.candidate.Requests)
		if granularity > length {
			granularity = length
		}
		chunkSize := (length + granularity - 1) / granularity
		reduced := false
		for start := 0; start < length && !state.exhausted; start += chunkSize {
			end := min(start+chunkSize, length)
			if end-start == length {
				continue
			}
			candidate := cloneCounterexample(state.candidate)
			candidate.Requests = append(slices.Clone(candidate.Requests[:start]), candidate.Requests[end:]...)
			if state.try(fmt.Sprintf("remove-requests-%d-%d", start, end), candidate) {
				granularity = max(2, granularity-1)
				reduced = true
				break
			}
		}
		if reduced {
			continue
		}
		if granularity == length {
			break
		}
		granularity = min(length, granularity*2)
	}
}

func (state *minimizer) minimizeConcurrency() {
	for !state.exhausted {
		current := uint64(state.candidate.Concurrency)
		changed := false
		for _, value := range reductionCandidates(current, 1) {
			candidate := cloneCounterexample(state.candidate)
			candidate.Concurrency = uint32(value)
			if state.try(fmt.Sprintf("set-concurrency-%d", value), candidate) {
				changed = true
				break
			}
		}
		if !changed {
			return
		}
	}
}

func (state *minimizer) minimizeFault() {
	switch state.candidate.Fault.Type {
	case FaultReplicaLoss:
		state.reduceUint("fault-duration", 1,
			func() uint64 { return uint64(state.candidate.Fault.DurationSeconds) },
			func(candidate *Counterexample, value uint64) { candidate.Fault.DurationSeconds = uint32(value) })
	case FaultLongContextSpike:
		state.reduceUint("long-context-tokens", 4096,
			func() uint64 { return state.candidate.Fault.LongContextTokens },
			func(candidate *Counterexample, value uint64) { candidate.Fault.LongContextTokens = value })
		for !state.exhausted {
			current := state.candidate.Fault.LongContextFraction
			changed := false
			for _, value := range floatReductionCandidates(current) {
				candidate := cloneCounterexample(state.candidate)
				candidate.Fault.LongContextFraction = value
				if state.try(fmt.Sprintf("set-long-context-fraction-%.6f", value), candidate) {
					changed = true
					break
				}
			}
			if !changed {
				break
			}
		}
	}
}

func (state *minimizer) minimizeRequestShapes() {
	for index := 0; index < len(state.candidate.Requests) && !state.exhausted; index++ {
		index := index
		state.reduceUint(fmt.Sprintf("request-%d-prompt", index), 1,
			func() uint64 { return state.candidate.Requests[index].PromptTokens },
			func(candidate *Counterexample, value uint64) { candidate.Requests[index].PromptTokens = value })
		state.reduceUint(fmt.Sprintf("request-%d-output", index), 1,
			func() uint64 { return state.candidate.Requests[index].OutputTokens },
			func(candidate *Counterexample, value uint64) { candidate.Requests[index].OutputTokens = value })
		minimumArrival := uint64(0)
		if index > 0 {
			minimumArrival = state.candidate.Requests[index-1].ArrivalOffsetMilliseconds
		}
		state.reduceUint(fmt.Sprintf("request-%d-arrival", index), minimumArrival,
			func() uint64 { return state.candidate.Requests[index].ArrivalOffsetMilliseconds },
			func(candidate *Counterexample, value uint64) {
				candidate.Requests[index].ArrivalOffsetMilliseconds = value
			})
	}
}

func (state *minimizer) reduceUint(label string, minimum uint64, current func() uint64, set func(*Counterexample, uint64)) {
	for !state.exhausted {
		changed := false
		for _, value := range reductionCandidates(current(), minimum) {
			candidate := cloneCounterexample(state.candidate)
			set(&candidate, value)
			if state.try(fmt.Sprintf("set-%s-%d", label, value), candidate) {
				changed = true
				break
			}
		}
		if !changed {
			return
		}
	}
}

func reductionCandidates(current, minimum uint64) []uint64 {
	if current <= minimum {
		return nil
	}
	values := []uint64{minimum}
	for value := uint64(1); value < current && value <= 1<<62; value *= 2 {
		if value >= minimum {
			values = append(values, value)
		}
	}
	values = append(values, minimum+(current-minimum)/2, current-1)
	slices.Sort(values)
	values = slices.Compact(values)
	result := values[:0]
	for _, value := range values {
		if value >= minimum && value < current {
			result = append(result, value)
		}
	}
	return result
}

func floatReductionCandidates(current float64) []float64 {
	if current <= 0.01 {
		return nil
	}
	result := make([]float64, 0, 100)
	for basisPoints := 1; basisPoints < 100; basisPoints++ {
		value := float64(basisPoints) / 100
		if value >= current {
			break
		}
		result = append(result, value)
	}
	return result
}

func cloneCounterexample(counterexample Counterexample) Counterexample {
	clone := counterexample
	clone.Requests = slices.Clone(counterexample.Requests)
	return clone
}
