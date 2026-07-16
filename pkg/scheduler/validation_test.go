package scheduler

import (
	"errors"
	"math"
	"testing"
	"time"
)

func TestValidateRequest(t *testing.T) {
	t.Parallel()

	valid := testRequest()
	if err := ValidateRequest(valid); err != nil {
		t.Fatalf("ValidateRequest() unexpected error: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Request)
	}{
		{name: "missing ID", mutate: func(r *Request) { r.ID = "" }},
		{name: "missing tenant", mutate: func(r *Request) { r.Tenant = "" }},
		{name: "missing model", mutate: func(r *Request) { r.Model = "" }},
		{name: "missing input tokens", mutate: func(r *Request) { r.InputTokens = 0 }},
		{name: "missing output tokens", mutate: func(r *Request) { r.MaxOutputTokens = 0 }},
		{name: "invalid priority", mutate: func(r *Request) { r.Priority = 101 }},
		{name: "missing arrival", mutate: func(r *Request) { r.ArrivalTime = time.Time{} }},
		{name: "invalid deadline", mutate: func(r *Request) { r.Deadline = r.ArrivalTime }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := valid
			tt.mutate(&req)
			if err := ValidateRequest(req); !errors.Is(err, ErrInvalidRequest) {
				t.Fatalf("ValidateRequest() error = %v, want ErrInvalidRequest", err)
			}
		})
	}
}

func TestValidateSnapshot(t *testing.T) {
	t.Parallel()

	valid := testSnapshot()
	if err := ValidateSnapshot(valid); err != nil {
		t.Fatalf("ValidateSnapshot() unexpected error: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*ClusterSnapshot)
	}{
		{name: "zero version", mutate: func(s *ClusterSnapshot) { s.Version = 0 }},
		{name: "missing capture time", mutate: func(s *ClusterSnapshot) { s.CapturedAt = time.Time{} }},
		{name: "no endpoints", mutate: func(s *ClusterSnapshot) { s.Endpoints = nil }},
		{name: "duplicate endpoint", mutate: func(s *ClusterSnapshot) { s.Endpoints = append(s.Endpoints, s.Endpoints[0]) }},
		{name: "no models", mutate: func(s *ClusterSnapshot) { s.Endpoints[0].Models = nil }},
		{name: "duplicate model", mutate: func(s *ClusterSnapshot) { s.Endpoints[0].Models = []string{"model", "model"} }},
		{name: "zero capacity", mutate: func(s *ClusterSnapshot) { s.Endpoints[0].MaxConcurrency = 0 }},
		{name: "over capacity", mutate: func(s *ClusterSnapshot) { s.Endpoints[0].ActiveRequests = 3 }},
		{name: "invalid throughput", mutate: func(s *ClusterSnapshot) { s.Endpoints[0].EstimatedTokensPerSecond = math.NaN() }},
		{name: "invalid cached prefix", mutate: func(s *ClusterSnapshot) { s.Endpoints[0].CachedPrefixes = map[string]float64{"prefix": 2} }},
		{name: "invalid adapter", mutate: func(s *ClusterSnapshot) { s.Endpoints[0].LoadedAdapters = map[string]struct{}{"": struct{}{}} }},
		{name: "zero state version", mutate: func(s *ClusterSnapshot) { s.Endpoints[0].StateVersion = 0 }},
		{name: "missing observation", mutate: func(s *ClusterSnapshot) { s.Endpoints[0].ObservedAt = time.Time{} }},
		{name: "future observation", mutate: func(s *ClusterSnapshot) { s.Endpoints[0].ObservedAt = s.CapturedAt.Add(time.Nanosecond) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			snapshot := testSnapshot()
			tt.mutate(&snapshot)
			if err := ValidateSnapshot(snapshot); !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("ValidateSnapshot() error = %v, want ErrInvalidSnapshot", err)
			}
		})
	}
}

func TestValidateDecision(t *testing.T) {
	t.Parallel()

	req := testRequest()
	snapshot := testSnapshot()
	valid := Decision{
		EndpointID:      "worker-a",
		Score:           1,
		SnapshotVersion: snapshot.Version,
		Reasons:         []Reason{{Code: "test", Message: "selected for test", Contribution: 1}},
	}
	if err := ValidateDecision(req, snapshot, valid); err != nil {
		t.Fatalf("ValidateDecision() unexpected error: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Decision)
	}{
		{name: "missing endpoint", mutate: func(d *Decision) { d.EndpointID = "" }},
		{name: "wrong snapshot", mutate: func(d *Decision) { d.SnapshotVersion++ }},
		{name: "non-finite score", mutate: func(d *Decision) { d.Score = math.Inf(1) }},
		{name: "no explanation", mutate: func(d *Decision) { d.Reasons = nil }},
		{name: "invalid reason", mutate: func(d *Decision) { d.Reasons[0].Code = "" }},
		{name: "ineligible endpoint", mutate: func(d *Decision) { d.EndpointID = "worker-b" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			decision := valid
			decision.Reasons = append([]Reason(nil), valid.Reasons...)
			tt.mutate(&decision)
			if err := ValidateDecision(req, snapshot, decision); !errors.Is(err, ErrInvalidDecision) {
				t.Fatalf("ValidateDecision() error = %v, want ErrInvalidDecision", err)
			}
		})
	}
}

func TestEligibleEndpoints(t *testing.T) {
	t.Parallel()

	req := testRequest()
	snapshot := testSnapshot()
	snapshot.Endpoints = []Endpoint{
		{ID: "saturated", Models: []string{"model"}, Healthy: true, MaxConcurrency: 1, ActiveRequests: 1},
		{ID: "draining", Models: []string{"model"}, Healthy: true, Draining: true, MaxConcurrency: 1},
		{ID: "unhealthy", Models: []string{"model"}, MaxConcurrency: 1},
		{ID: "wrong-model", Models: []string{"other-model"}, Healthy: true, MaxConcurrency: 1},
	}

	got := EligibleEndpoints(req, snapshot)
	if len(got) != 1 || got[0].ID != "saturated" {
		t.Fatalf("EligibleEndpoints() = %v, want only saturated endpoint", got)
	}
}

func testRequest() Request {
	arrival := time.Unix(1_700_000_000, 0).UTC()
	return Request{
		ID:              "request-1",
		Tenant:          "tenant-a",
		Model:           "model",
		InputTokens:     128,
		MaxOutputTokens: 64,
		ArrivalTime:     arrival,
		Deadline:        arrival.Add(time.Second),
	}
}

func testSnapshot() ClusterSnapshot {
	now := time.Unix(1_700_000_000, 0).UTC()
	return ClusterSnapshot{
		Version:    7,
		CapturedAt: now,
		Endpoints: []Endpoint{
			{
				ID:                       "worker-a",
				Models:                   []string{"model"},
				Healthy:                  true,
				MaxConcurrency:           2,
				EstimatedTokensPerSecond: 100,
				StateVersion:             3,
				ObservedAt:               now,
			},
			{
				ID:                       "worker-b",
				Models:                   []string{"other-model"},
				Healthy:                  true,
				MaxConcurrency:           2,
				EstimatedTokensPerSecond: 100,
				StateVersion:             2,
				ObservedAt:               now,
			},
		},
	}
}
