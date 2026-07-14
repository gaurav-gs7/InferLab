package evidence

import (
	"fmt"
	"slices"
	"strings"
	"sync"
)

type NodeKind string

const (
	NodeRuntime     NodeKind = "runtime"
	NodeWorkload    NodeKind = "workload"
	NodeArtifact    NodeKind = "artifact"
	NodeEvidence    NodeKind = "evidence"
	NodeCalibration NodeKind = "calibration"
	NodePolicy      NodeKind = "policy"
	NodeClaim       NodeKind = "claim"
	NodeDecision    NodeKind = "decision"
)

type Node struct {
	ID        string
	Kind      NodeKind
	DependsOn []string
}

type Invalidation struct {
	NodeID      string   `json:"node_id"`
	RootID      string   `json:"root_id"`
	Path        []string `json:"path"`
	Reason      string   `json:"reason"`
	Explanation string   `json:"explanation"`
}

// ValidityGraph is an append-only dependency DAG. Requiring dependencies to
// exist before Add makes cycles impossible and keeps propagation deterministic.
type ValidityGraph struct {
	mu      sync.RWMutex
	nodes   map[string]Node
	reverse map[string][]string
}

func NewValidityGraph() *ValidityGraph {
	return &ValidityGraph{
		nodes:   make(map[string]Node),
		reverse: make(map[string][]string),
	}
}

func (graph *ValidityGraph) Add(node Node) error {
	if graph == nil {
		return fmt.Errorf("%w: graph is nil", ErrInvalidGraph)
	}
	if !namePattern.MatchString(node.ID) && !digestPattern.MatchString(node.ID) {
		return fmt.Errorf("%w: node id %q is invalid", ErrInvalidGraph, node.ID)
	}
	if !validNodeKind(node.Kind) {
		return fmt.Errorf("%w: node kind %q is invalid", ErrInvalidGraph, node.Kind)
	}
	graph.mu.Lock()
	defer graph.mu.Unlock()
	if _, exists := graph.nodes[node.ID]; exists {
		return fmt.Errorf("%w: duplicate node %q", ErrInvalidGraph, node.ID)
	}
	dependencies := slices.Clone(node.DependsOn)
	slices.Sort(dependencies)
	for i, dependency := range dependencies {
		if dependency == node.ID {
			return fmt.Errorf("%w: node %q depends on itself", ErrInvalidGraph, node.ID)
		}
		if i > 0 && dependency == dependencies[i-1] {
			return fmt.Errorf("%w: node %q repeats dependency %q", ErrInvalidGraph, node.ID, dependency)
		}
		if _, exists := graph.nodes[dependency]; !exists {
			return fmt.Errorf("%w: dependency %q does not exist", ErrInvalidGraph, dependency)
		}
	}
	node.DependsOn = dependencies
	graph.nodes[node.ID] = node
	for _, dependency := range dependencies {
		graph.reverse[dependency] = append(graph.reverse[dependency], node.ID)
		slices.Sort(graph.reverse[dependency])
	}
	return nil
}

// Invalidate returns each affected node once with its deterministic shortest
// dependency path from root.
func (graph *ValidityGraph) Invalidate(rootID, reason string) ([]Invalidation, error) {
	if graph == nil {
		return nil, fmt.Errorf("%w: graph is nil", ErrInvalidGraph)
	}
	if err := validateRequiredIdentifier("reason", reason, ErrInvalidGraph); err != nil {
		return nil, err
	}
	graph.mu.RLock()
	defer graph.mu.RUnlock()
	if _, exists := graph.nodes[rootID]; !exists {
		return nil, fmt.Errorf("%w: root node %q does not exist", ErrInvalidGraph, rootID)
	}

	type queued struct {
		id   string
		path []string
	}
	queue := []queued{{id: rootID, path: []string{rootID}}}
	visited := make(map[string]struct{}, len(graph.nodes))
	result := make([]Invalidation, 0)
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if _, exists := visited[current.id]; exists {
			continue
		}
		visited[current.id] = struct{}{}
		path := slices.Clone(current.path)
		result = append(result, Invalidation{
			NodeID:      current.id,
			RootID:      rootID,
			Path:        path,
			Reason:      reason,
			Explanation: strings.Join(path, " -> ") + ": " + reason,
		})
		for _, dependent := range graph.reverse[current.id] {
			dependentPath := append(slices.Clone(path), dependent)
			queue = append(queue, queued{id: dependent, path: dependentPath})
		}
	}
	return result, nil
}

func validNodeKind(kind NodeKind) bool {
	switch kind {
	case NodeRuntime, NodeWorkload, NodeArtifact, NodeEvidence, NodeCalibration, NodePolicy, NodeClaim, NodeDecision:
		return true
	default:
		return false
	}
}
