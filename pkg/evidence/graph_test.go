package evidence

import (
	"errors"
	"reflect"
	"sync"
	"testing"
)

func TestValidityGraphTransitiveInvalidationUsesShortestPath(t *testing.T) {
	t.Parallel()
	graph := NewValidityGraph()
	nodes := []Node{
		{ID: "runtime", Kind: NodeRuntime},
		{ID: "observation", Kind: NodeEvidence, DependsOn: []string{"runtime"}},
		{ID: "calibration", Kind: NodeCalibration, DependsOn: []string{"observation"}},
		{ID: "claim", Kind: NodeClaim, DependsOn: []string{"calibration", "observation"}},
		{ID: "decision", Kind: NodeDecision, DependsOn: []string{"claim"}},
	}
	for _, node := range nodes {
		if err := graph.Add(node); err != nil {
			t.Fatal(err)
		}
	}
	invalidations, err := graph.Invalidate("runtime", "container digest changed")
	if err != nil {
		t.Fatal(err)
	}
	if len(invalidations) != len(nodes) {
		t.Fatalf("got %d invalidations, want %d", len(invalidations), len(nodes))
	}
	var claim Invalidation
	for _, invalidation := range invalidations {
		if invalidation.NodeID == "claim" {
			claim = invalidation
		}
	}
	want := []string{"runtime", "observation", "claim"}
	if !reflect.DeepEqual(claim.Path, want) {
		t.Fatalf("claim path = %v, want %v", claim.Path, want)
	}
	if claim.Explanation != "runtime -> observation -> claim: container digest changed" {
		t.Fatalf("explanation = %q", claim.Explanation)
	}
}

func TestValidityGraphConcurrentInvalidation(t *testing.T) {
	t.Parallel()
	graph := NewValidityGraph()
	if err := graph.Add(Node{ID: "runtime", Kind: NodeRuntime}); err != nil {
		t.Fatal(err)
	}
	if err := graph.Add(Node{ID: "claim", Kind: NodeClaim, DependsOn: []string{"runtime"}}); err != nil {
		t.Fatal(err)
	}
	var wait sync.WaitGroup
	for range 16 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			invalidations, err := graph.Invalidate("runtime", "changed")
			if err != nil || len(invalidations) != 2 {
				t.Errorf("Invalidate() = %d, %v", len(invalidations), err)
			}
		}()
	}
	wait.Wait()
}

func TestValidityGraphRejectsInvalidNodes(t *testing.T) {
	t.Parallel()
	graph := NewValidityGraph()
	if err := graph.Add(Node{ID: "evidence", Kind: NodeEvidence, DependsOn: []string{"missing"}}); !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidGraph)
	}
	if _, err := graph.Invalidate("missing", "stale"); !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidGraph)
	}
	if err := graph.Add(Node{ID: "root", Kind: NodeRuntime}); err != nil {
		t.Fatal(err)
	}
	tests := []Node{
		{ID: "Invalid", Kind: NodeEvidence},
		{ID: "bad-kind", Kind: "other"},
		{ID: "root", Kind: NodeRuntime},
		{ID: "self", Kind: NodeEvidence, DependsOn: []string{"self"}},
		{ID: "repeat", Kind: NodeEvidence, DependsOn: []string{"root", "root"}},
	}
	for _, node := range tests {
		if err := graph.Add(node); !errors.Is(err, ErrInvalidGraph) {
			t.Fatalf("Add(%#v) error = %v, want %v", node, err, ErrInvalidGraph)
		}
	}
	var nilGraph *ValidityGraph
	if err := nilGraph.Add(Node{}); !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("nil Add error = %v", err)
	}
	if _, err := nilGraph.Invalidate("root", "stale"); !errors.Is(err, ErrInvalidGraph) {
		t.Fatalf("nil Invalidate error = %v", err)
	}
}
