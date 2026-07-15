package gate

import (
	"slices"
	"strings"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

func buildGraph(evaluation Evaluation, evaluationDigest string, indexed []indexedEvidence, findings []Finding) ([]GraphNode, error) {
	graph := evidence.NewValidityGraph()
	nodes := make([]GraphNode, 0)
	added := make(map[string]struct{})
	add := func(node GraphNode) error {
		if _, exists := added[node.ID]; exists {
			return nil
		}
		node.DependsOn = slices.Clone(node.DependsOn)
		slices.Sort(node.DependsOn)
		if err := graph.Add(evidence.Node{ID: node.ID, Kind: node.Kind, DependsOn: node.DependsOn}); err != nil {
			return err
		}
		added[node.ID] = struct{}{}
		nodes = append(nodes, node)
		return nil
	}

	targetDigest, err := evidence.RuntimeSignatureDigest(evaluation.TargetRuntime)
	if err != nil {
		return nil, err
	}
	runtimeDigests := map[string]struct{}{targetDigest: {}}
	for _, entry := range indexed {
		runtimeDigests[entry.runtimeDigest] = struct{}{}
	}
	for _, digest := range sortedKeys(runtimeDigests) {
		if err := add(GraphNode{ID: graphID("runtime", digest), Kind: evidence.NodeRuntime, DependsOn: []string{}}); err != nil {
			return nil, err
		}
	}

	workloadDigests := make(map[string]struct{})
	for _, region := range evaluation.Regions {
		workloadDigests[region.WorkloadDigest] = struct{}{}
	}
	for _, entry := range indexed {
		workloadDigests[entry.node.Envelope.WorkloadDigest] = struct{}{}
	}
	for _, digest := range sortedKeys(workloadDigests) {
		if err := add(GraphNode{ID: graphID("workload", digest), Kind: evidence.NodeWorkload, DependsOn: []string{}}); err != nil {
			return nil, err
		}
	}

	artifacts := make(map[string]struct{})
	for _, entry := range indexed {
		for _, artifact := range entry.node.Envelope.Artifacts {
			artifacts[artifact.Digest] = struct{}{}
		}
	}
	for _, digest := range sortedKeys(artifacts) {
		if err := add(GraphNode{ID: graphID("artifact", digest), Kind: evidence.NodeArtifact, DependsOn: []string{}}); err != nil {
			return nil, err
		}
	}

	for _, entry := range indexed {
		dependencies := []string{graphID("runtime", entry.runtimeDigest), graphID("workload", entry.node.Envelope.WorkloadDigest)}
		for _, artifact := range entry.node.Envelope.Artifacts {
			dependencies = append(dependencies, graphID("artifact", artifact.Digest))
		}
		if err := add(GraphNode{ID: graphID("evidence", entry.digest), Kind: evidence.NodeEvidence, DependsOn: dependencies}); err != nil {
			return nil, err
		}
	}
	evaluationDependencies := []string{graphID("runtime", targetDigest)}
	for _, region := range evaluation.Regions {
		evaluationDependencies = append(evaluationDependencies, graphID("workload", region.WorkloadDigest))
	}
	for _, entry := range indexed {
		evaluationDependencies = append(evaluationDependencies, graphID("evidence", entry.digest))
	}
	slices.Sort(evaluationDependencies)
	evaluationDependencies = slices.Compact(evaluationDependencies)
	evaluationNodeID := graphID("claim", evaluationDigest)
	if err := add(GraphNode{ID: evaluationNodeID, Kind: evidence.NodeClaim, DependsOn: evaluationDependencies}); err != nil {
		return nil, err
	}

	regions := make(map[string]Region, len(evaluation.Regions))
	for _, region := range evaluation.Regions {
		regions[region.Name] = region
	}
	rules := slices.Clone(evaluation.Rules)
	slices.SortFunc(rules, func(a, b Rule) int { return strings.Compare(a.ID, b.ID) })
	policyIDs := make([]string, 0, len(rules))
	for _, rule := range rules {
		dependencies := []string{evaluationNodeID, graphID("runtime", targetDigest), graphID("workload", regions[rule.Region].WorkloadDigest)}
		for _, finding := range findings {
			if finding.RuleID == rule.ID && finding.EvidenceDigest != "" {
				dependencies = append(dependencies, graphID("evidence", finding.EvidenceDigest))
			}
		}
		slices.Sort(dependencies)
		dependencies = slices.Compact(dependencies)
		policyID := "policy-" + rule.ID
		if err := add(GraphNode{ID: policyID, Kind: evidence.NodePolicy, DependsOn: dependencies}); err != nil {
			return nil, err
		}
		policyIDs = append(policyIDs, policyID)
	}
	slices.Sort(policyIDs)
	if err := add(GraphNode{ID: "decision-" + evaluation.Name, Kind: evidence.NodeDecision, DependsOn: policyIDs}); err != nil {
		return nil, err
	}
	return nodes, nil
}

func graphID(prefix, digest string) string {
	return prefix + "-" + strings.TrimPrefix(digest, "sha256:")
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
