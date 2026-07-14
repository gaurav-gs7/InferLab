// Package roundrobin provides the deterministic baseline scheduling policy.
package roundrobin

import (
	"context"
	"fmt"
	"sort"
	"sync/atomic"

	"github.com/gaurav-gs7/InferLab/pkg/scheduler"
)

// Scheduler selects eligible endpoints in stable ID order. The policy is safe
// for concurrent use; replay callers should create a fresh instance per run.
type Scheduler struct {
	next atomic.Uint64
}

// New returns a round-robin scheduler with an empty cursor.
func New() *Scheduler {
	return &Scheduler{}
}

// Name implements scheduler.Scheduler.
func (*Scheduler) Name() string {
	return "round-robin"
}

// Select implements scheduler.Scheduler.
func (s *Scheduler) Select(ctx context.Context, req scheduler.Request, snapshot scheduler.ClusterSnapshot) (scheduler.Decision, error) {
	if err := ctx.Err(); err != nil {
		return scheduler.Decision{}, err
	}
	if err := scheduler.ValidateRequest(req); err != nil {
		return scheduler.Decision{}, err
	}
	if err := scheduler.ValidateSnapshot(snapshot); err != nil {
		return scheduler.Decision{}, err
	}

	eligible := scheduler.EligibleEndpoints(req, snapshot)
	if len(eligible) == 0 {
		return scheduler.Decision{}, fmt.Errorf("%w for model %q", scheduler.ErrNoEligibleEndpoints, req.Model)
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].ID < eligible[j].ID })
	if err := ctx.Err(); err != nil {
		return scheduler.Decision{}, err
	}

	index := (s.next.Add(1) - 1) % uint64(len(eligible))
	selected := eligible[index]
	decision := scheduler.Decision{
		EndpointID:      selected.ID,
		Score:           0,
		SnapshotVersion: snapshot.Version,
		Reasons: []scheduler.Reason{
			{
				Code:         "round_robin_turn",
				Message:      "selected by the deterministic round-robin cursor",
				Contribution: 0,
				Metadata: map[string]string{
					"candidate_count": fmt.Sprint(len(eligible)),
					"cursor_index":    fmt.Sprint(index),
				},
			},
		},
	}
	if err := scheduler.ValidateDecision(req, snapshot, decision); err != nil {
		return scheduler.Decision{}, err
	}
	return decision, nil
}

var _ scheduler.Scheduler = (*Scheduler)(nil)
