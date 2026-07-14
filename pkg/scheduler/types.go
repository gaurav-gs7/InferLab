// Package scheduler defines the stable contracts used by InferLab scheduling
// policies, deterministic replay, and production shadow evaluation.
package scheduler

import (
	"context"
	"time"
)

// Scheduler selects one endpoint from a point-in-time cluster snapshot.
//
// Implementations must be safe for concurrent use, must honor context
// cancellation, and must not mutate req or cluster. A successful decision must
// pass ValidateDecision.
type Scheduler interface {
	Name() string
	Select(ctx context.Context, req Request, cluster ClusterSnapshot) (Decision, error)
}

// Request contains only the metadata needed to make a scheduling decision.
// Raw prompt or response content does not belong in this type.
type Request struct {
	ID                string
	Tenant            string
	Model             string
	InputTokens       uint64
	MaxOutputTokens   uint64
	PrefixFingerprint string
	Adapter           string
	Priority          uint8
	ArrivalTime       time.Time
	Deadline          time.Time
}

// Endpoint is the scheduler-visible state of one inference worker.
type Endpoint struct {
	ID                       string
	Models                   []string
	Healthy                  bool
	Draining                 bool
	MaxConcurrency           uint32
	ActiveRequests           uint32
	QueuedTokens             uint64
	EstimatedTokensPerSecond float64
	CachedPrefixes           map[string]float64
	LoadedAdapters           map[string]struct{}
	StateVersion             uint64
	ObservedAt               time.Time
}

// ClusterSnapshot is a versioned, point-in-time view of candidate endpoints.
// Policies must treat the snapshot and all nested values as immutable.
type ClusterSnapshot struct {
	Version    uint64
	CapturedAt time.Time
	Endpoints  []Endpoint
}

// Decision is an explainable endpoint selection made against one snapshot.
type Decision struct {
	EndpointID      string
	Score           float64
	Reasons         []Reason
	SnapshotVersion uint64
}

// Reason explains one factor in a scheduling decision. Contribution uses the
// convention that higher values favor the selected endpoint.
type Reason struct {
	Code         string
	Message      string
	Contribution float64
	Metadata     map[string]string
}
