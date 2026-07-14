package scheduler

import (
	"errors"
	"fmt"
	"math"
	"slices"
)

var (
	// ErrNoEligibleEndpoints indicates that no endpoint can serve the request.
	ErrNoEligibleEndpoints = errors.New("no eligible endpoints")
	// ErrInvalidRequest indicates malformed scheduler input.
	ErrInvalidRequest = errors.New("invalid request")
	// ErrInvalidSnapshot indicates malformed or internally inconsistent state.
	ErrInvalidSnapshot = errors.New("invalid cluster snapshot")
	// ErrInvalidDecision indicates a scheduler contract violation.
	ErrInvalidDecision = errors.New("invalid scheduler decision")
)

// ValidateRequest checks invariants required by every scheduling policy.
func ValidateRequest(req Request) error {
	switch {
	case req.ID == "":
		return fmt.Errorf("%w: ID is required", ErrInvalidRequest)
	case req.Tenant == "":
		return fmt.Errorf("%w: tenant is required", ErrInvalidRequest)
	case req.Model == "":
		return fmt.Errorf("%w: model is required", ErrInvalidRequest)
	case req.InputTokens == 0:
		return fmt.Errorf("%w: input tokens must be greater than zero", ErrInvalidRequest)
	case req.MaxOutputTokens == 0:
		return fmt.Errorf("%w: maximum output tokens must be greater than zero", ErrInvalidRequest)
	case req.Priority > 100:
		return fmt.Errorf("%w: priority must be between 0 and 100", ErrInvalidRequest)
	case req.ArrivalTime.IsZero():
		return fmt.Errorf("%w: arrival time is required", ErrInvalidRequest)
	case !req.Deadline.IsZero() && !req.Deadline.After(req.ArrivalTime):
		return fmt.Errorf("%w: deadline must be after arrival time", ErrInvalidRequest)
	default:
		return nil
	}
}

// ValidateSnapshot checks identity, capacity, model, and time invariants.
func ValidateSnapshot(snapshot ClusterSnapshot) error {
	if snapshot.Version == 0 {
		return fmt.Errorf("%w: version must be greater than zero", ErrInvalidSnapshot)
	}
	if snapshot.CapturedAt.IsZero() {
		return fmt.Errorf("%w: capture time is required", ErrInvalidSnapshot)
	}
	if len(snapshot.Endpoints) == 0 {
		return fmt.Errorf("%w: at least one endpoint is required", ErrInvalidSnapshot)
	}

	seen := make(map[string]struct{}, len(snapshot.Endpoints))
	for i, endpoint := range snapshot.Endpoints {
		if endpoint.ID == "" {
			return fmt.Errorf("%w: endpoint %d has no ID", ErrInvalidSnapshot, i)
		}
		if _, exists := seen[endpoint.ID]; exists {
			return fmt.Errorf("%w: duplicate endpoint ID %q", ErrInvalidSnapshot, endpoint.ID)
		}
		seen[endpoint.ID] = struct{}{}

		if len(endpoint.Models) == 0 {
			return fmt.Errorf("%w: endpoint %q has no models", ErrInvalidSnapshot, endpoint.ID)
		}
		models := make(map[string]struct{}, len(endpoint.Models))
		for _, model := range endpoint.Models {
			if model == "" {
				return fmt.Errorf("%w: endpoint %q has an empty model", ErrInvalidSnapshot, endpoint.ID)
			}
			if _, exists := models[model]; exists {
				return fmt.Errorf("%w: endpoint %q repeats model %q", ErrInvalidSnapshot, endpoint.ID, model)
			}
			models[model] = struct{}{}
		}
		if endpoint.MaxConcurrency == 0 {
			return fmt.Errorf("%w: endpoint %q has zero maximum concurrency", ErrInvalidSnapshot, endpoint.ID)
		}
		if endpoint.ActiveRequests > endpoint.MaxConcurrency {
			return fmt.Errorf("%w: endpoint %q active requests exceed maximum concurrency", ErrInvalidSnapshot, endpoint.ID)
		}
		if endpoint.EstimatedTokensPerSecond < 0 || math.IsNaN(endpoint.EstimatedTokensPerSecond) || math.IsInf(endpoint.EstimatedTokensPerSecond, 0) {
			return fmt.Errorf("%w: endpoint %q has invalid token throughput", ErrInvalidSnapshot, endpoint.ID)
		}
		for fingerprint, fraction := range endpoint.CachedPrefixes {
			if fingerprint == "" || fraction < 0 || fraction > 1 || math.IsNaN(fraction) || math.IsInf(fraction, 0) {
				return fmt.Errorf("%w: endpoint %q has invalid cached prefix data", ErrInvalidSnapshot, endpoint.ID)
			}
		}
		for adapter := range endpoint.LoadedAdapters {
			if adapter == "" {
				return fmt.Errorf("%w: endpoint %q has an empty adapter", ErrInvalidSnapshot, endpoint.ID)
			}
		}
		if endpoint.StateVersion == 0 {
			return fmt.Errorf("%w: endpoint %q has zero state version", ErrInvalidSnapshot, endpoint.ID)
		}
		if endpoint.ObservedAt.IsZero() {
			return fmt.Errorf("%w: endpoint %q has no observation time", ErrInvalidSnapshot, endpoint.ID)
		}
	}
	return nil
}

// EligibleEndpoints returns healthy, non-draining endpoints that serve the
// requested model. Saturated endpoints remain eligible because flow control or
// the worker may queue a request. The returned pointers refer to the supplied
// snapshot and must not be mutated.
func EligibleEndpoints(req Request, snapshot ClusterSnapshot) []*Endpoint {
	eligible := make([]*Endpoint, 0, len(snapshot.Endpoints))
	for i := range snapshot.Endpoints {
		endpoint := &snapshot.Endpoints[i]
		if endpoint.Healthy && !endpoint.Draining && slices.Contains(endpoint.Models, req.Model) {
			eligible = append(eligible, endpoint)
		}
	}
	return eligible
}

// ValidateDecision checks that a policy returned an eligible endpoint and a
// complete, finite explanation for the snapshot it evaluated.
func ValidateDecision(req Request, snapshot ClusterSnapshot, decision Decision) error {
	if decision.EndpointID == "" {
		return fmt.Errorf("%w: endpoint ID is required", ErrInvalidDecision)
	}
	if decision.SnapshotVersion != snapshot.Version {
		return fmt.Errorf("%w: snapshot version %d does not match %d", ErrInvalidDecision, decision.SnapshotVersion, snapshot.Version)
	}
	if math.IsNaN(decision.Score) || math.IsInf(decision.Score, 0) {
		return fmt.Errorf("%w: score must be finite", ErrInvalidDecision)
	}
	if len(decision.Reasons) == 0 {
		return fmt.Errorf("%w: at least one reason is required", ErrInvalidDecision)
	}
	for i, reason := range decision.Reasons {
		if reason.Code == "" || reason.Message == "" {
			return fmt.Errorf("%w: reason %d requires code and message", ErrInvalidDecision, i)
		}
		if math.IsNaN(reason.Contribution) || math.IsInf(reason.Contribution, 0) {
			return fmt.Errorf("%w: reason %d contribution must be finite", ErrInvalidDecision, i)
		}
	}

	for _, endpoint := range EligibleEndpoints(req, snapshot) {
		if endpoint.ID == decision.EndpointID {
			return nil
		}
	}
	return fmt.Errorf("%w: endpoint %q is not eligible", ErrInvalidDecision, decision.EndpointID)
}
