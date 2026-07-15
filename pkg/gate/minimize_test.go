package gate

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestMinimizeDeterministicAndReverified(t *testing.T) {
	t.Parallel()
	initial := failingCounterexample()
	oracle := func(_ context.Context, candidate Counterexample) (bool, error) {
		if candidate.Concurrency < 3 || candidate.Fault.DurationSeconds < 5 {
			return false, nil
		}
		qualifying := 0
		for _, request := range candidate.Requests {
			if request.Tenant == "noisy" && request.PromptTokens >= 1024 {
				qualifying++
			}
		}
		return qualifying >= 2, nil
	}
	options := MinimizeOptions{MaximumEvaluations: 5000, VerificationRuns: 3}
	first, err := Minimize(context.Background(), initial, options, oracle)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Minimize(context.Background(), initial, options, oracle)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("minimization is not deterministic:\n%+v\n%+v", first, second)
	}
	if !first.SearchComplete || first.VerificationRuns != 3 || first.Counterexample.Concurrency != 3 || first.Counterexample.Fault.DurationSeconds != 5 || len(first.Counterexample.Requests) != 2 {
		t.Fatalf("unexpected minimized counterexample: %+v", first)
	}
	for _, request := range first.Counterexample.Requests {
		if request.Tenant != "noisy" || request.PromptTokens != 1024 || request.OutputTokens != 1 {
			t.Fatalf("request was not minimized: %+v", request)
		}
	}
	fails, err := oracle(context.Background(), first.Counterexample)
	if err != nil || !fails {
		t.Fatal("reported counterexample does not reproduce")
	}
}

func TestMinimizeBudgetAndReverificationFailures(t *testing.T) {
	t.Parallel()
	initial := failingCounterexample()
	t.Run("partial search is explicit", func(t *testing.T) {
		t.Parallel()
		result, err := Minimize(context.Background(), initial, MinimizeOptions{MaximumEvaluations: 3, VerificationRuns: 1}, func(context.Context, Counterexample) (bool, error) { return true, nil })
		if err != nil {
			t.Fatal(err)
		}
		if result.SearchComplete {
			t.Fatal("budget-exhausted search was reported complete")
		}
	})
	t.Run("non reproducing input", func(t *testing.T) {
		t.Parallel()
		_, err := Minimize(context.Background(), initial, MinimizeOptions{MaximumEvaluations: 10, VerificationRuns: 1}, func(context.Context, Counterexample) (bool, error) { return false, nil })
		if !errors.Is(err, ErrCounterexampleDoesNotFail) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("reverification catches instability", func(t *testing.T) {
		t.Parallel()
		calls := 0
		_, err := Minimize(context.Background(), initial, MinimizeOptions{MaximumEvaluations: 2, VerificationRuns: 1}, func(context.Context, Counterexample) (bool, error) {
			calls++
			return calls == 1, nil
		})
		if !errors.Is(err, ErrCounterexampleNotReproduced) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("cancellation", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := Minimize(ctx, initial, MinimizeOptions{MaximumEvaluations: 10, VerificationRuns: 1}, func(context.Context, Counterexample) (bool, error) { return true, nil })
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestMinimizeLongContextPressure(t *testing.T) {
	t.Parallel()
	initial := NewCounterexample()
	initial.Name = "long-context-failure"
	initial.Concurrency = 8
	initial.Fault = FaultPoint{Type: FaultLongContextSpike, LongContextTokens: 32768, LongContextFraction: 0.75}
	initial.Requests = []CounterexampleRequest{{ID: "request-1", PromptTokens: 8192, OutputTokens: 64, Tenant: "tenant-a"}}
	oracle := func(_ context.Context, candidate Counterexample) (bool, error) {
		return candidate.Fault.LongContextTokens >= 8192 && candidate.Fault.LongContextFraction >= 0.25, nil
	}
	result, err := Minimize(context.Background(), initial, MinimizeOptions{MaximumEvaluations: 1000, VerificationRuns: 2}, oracle)
	if err != nil {
		t.Fatal(err)
	}
	if !result.SearchComplete || result.Counterexample.Fault.LongContextTokens != 8192 || result.Counterexample.Fault.LongContextFraction != 0.25 {
		t.Fatalf("unexpected long-context minimum: %+v", result)
	}
}

func failingCounterexample() Counterexample {
	counterexample := NewCounterexample()
	counterexample.Name = "noisy-neighbor-failure"
	counterexample.Concurrency = 16
	counterexample.Fault = FaultPoint{Type: FaultReplicaLoss, LostReplicas: 1, DurationSeconds: 30}
	counterexample.Requests = []CounterexampleRequest{
		{ID: "request-1", ArrivalOffsetMilliseconds: 0, PromptTokens: 256, OutputTokens: 64, Tenant: "quiet"},
		{ID: "request-2", ArrivalOffsetMilliseconds: 10, PromptTokens: 8192, OutputTokens: 128, Tenant: "noisy"},
		{ID: "request-3", ArrivalOffsetMilliseconds: 20, PromptTokens: 4096, OutputTokens: 96, Tenant: "noisy"},
		{ID: "request-4", ArrivalOffsetMilliseconds: 30, PromptTokens: 128, OutputTokens: 32, Tenant: "quiet"},
		{ID: "request-5", ArrivalOffsetMilliseconds: 40, PromptTokens: 2048, OutputTokens: 48, Tenant: "noisy"},
		{ID: "request-6", ArrivalOffsetMilliseconds: 50, PromptTokens: 64, OutputTokens: 16, Tenant: "quiet"},
	}
	return counterexample
}
