package roundrobin

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gaurav-gs7/InferLab/pkg/scheduler"
)

func TestSchedulerCyclesInStableOrder(t *testing.T) {
	t.Parallel()

	policy := New()
	if got := policy.Name(); got != "round-robin" {
		t.Fatalf("Name() = %q, want round-robin", got)
	}
	snapshot := validSnapshot()
	snapshot.Endpoints[0], snapshot.Endpoints[1] = snapshot.Endpoints[1], snapshot.Endpoints[0]
	want := []string{"worker-a", "worker-b", "worker-a", "worker-b"}
	for i, endpointID := range want {
		decision, err := policy.Select(context.Background(), validRequest(), snapshot)
		if err != nil {
			t.Fatalf("Select() call %d error: %v", i, err)
		}
		if decision.EndpointID != endpointID {
			t.Fatalf("Select() call %d endpoint = %q, want %q", i, decision.EndpointID, endpointID)
		}
		if err := scheduler.ValidateDecision(validRequest(), snapshot, decision); err != nil {
			t.Fatalf("decision contract: %v", err)
		}
	}
}

func TestSchedulerRejectsInvalidInputs(t *testing.T) {
	t.Parallel()
	if _, err := New().Select(context.Background(), scheduler.Request{}, validSnapshot()); err == nil {
		t.Fatal("Select() accepted an invalid request")
	}
	if _, err := New().Select(context.Background(), validRequest(), scheduler.ClusterSnapshot{}); err == nil {
		t.Fatal("Select() accepted an invalid snapshot")
	}
}

func TestSchedulerFiltersIneligibleEndpoints(t *testing.T) {
	t.Parallel()

	snapshot := validSnapshot()
	snapshot.Endpoints[0].Healthy = false
	decision, err := New().Select(context.Background(), validRequest(), snapshot)
	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if decision.EndpointID != "worker-b" {
		t.Fatalf("Select() endpoint = %q, want worker-b", decision.EndpointID)
	}
}

func TestSchedulerNoEligibleEndpoint(t *testing.T) {
	t.Parallel()

	snapshot := validSnapshot()
	for i := range snapshot.Endpoints {
		snapshot.Endpoints[i].Draining = true
	}
	_, err := New().Select(context.Background(), validRequest(), snapshot)
	if !errors.Is(err, scheduler.ErrNoEligibleEndpoints) {
		t.Fatalf("Select() error = %v, want ErrNoEligibleEndpoints", err)
	}
}

func TestSchedulerHonorsCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := New().Select(ctx, validRequest(), validSnapshot())
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Select() error = %v, want context.Canceled", err)
	}
}

func TestSchedulerConcurrentDistribution(t *testing.T) {
	t.Parallel()

	const calls = 1_000
	policy := New()
	results := make(chan string, calls)
	var wg sync.WaitGroup
	for range calls {
		wg.Add(1)
		go func() {
			defer wg.Done()
			decision, err := policy.Select(context.Background(), validRequest(), validSnapshot())
			if err != nil {
				results <- "error"
				return
			}
			results <- decision.EndpointID
		}()
	}
	wg.Wait()
	close(results)

	counts := map[string]int{}
	for result := range results {
		counts[result]++
	}
	if counts["worker-a"] != calls/2 || counts["worker-b"] != calls/2 || counts["error"] != 0 {
		t.Fatalf("unexpected concurrent distribution: %v", counts)
	}
}

func validRequest() scheduler.Request {
	now := time.Unix(1_700_000_000, 0).UTC()
	return scheduler.Request{
		ID:              "request-1",
		Tenant:          "tenant",
		Model:           "model",
		InputTokens:     100,
		MaxOutputTokens: 20,
		ArrivalTime:     now,
		Deadline:        now.Add(time.Second),
	}
}

func validSnapshot() scheduler.ClusterSnapshot {
	now := time.Unix(1_700_000_000, 0).UTC()
	endpoint := func(id string) scheduler.Endpoint {
		return scheduler.Endpoint{
			ID:                       id,
			Models:                   []string{"model"},
			Healthy:                  true,
			MaxConcurrency:           4,
			EstimatedTokensPerSecond: 100,
			StateVersion:             1,
			ObservedAt:               now,
		}
	}
	return scheduler.ClusterSnapshot{
		Version:    1,
		CapturedAt: now,
		Endpoints:  []scheduler.Endpoint{endpoint("worker-a"), endpoint("worker-b")},
	}
}
