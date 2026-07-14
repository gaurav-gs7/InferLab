# Architecture

## Product boundary

InferLab is a scheduler development and validation framework. It does not proxy the production data path after a policy is accepted, implement an inference engine, manage model replicas, or replace the llm-d Router.

Its responsibility begins with privacy-safe scheduling metadata and ends with evidence plus an integration artifact:

1. capture a workload without raw prompt content by default;
2. normalize observations into a versioned trace;
3. replay the same arrivals against isolated policy instances;
4. model worker queues, prefill, decode, cancellation, cache state, and failures;
5. compute latency, fairness, utilization, cache, and cost metrics;
6. evaluate machine-readable correctness assertions;
7. shadow a candidate beside production without affecting routing;
8. export validated configuration or plugin adapters to a serving system.

## Components

```text
┌──────────────────┐     ┌────────────────────┐
│ Capture adapters │────▶│ Versioned trace log │
│ OpenAI / Router  │     │ metadata only       │
└──────────────────┘     └─────────┬──────────┘
                                   │
                                   ▼
                         ┌─────────────────────┐
                         │ Deterministic engine│
                         │ virtual clock/events│
                         └──────┬──────────────┘
                                │ snapshots
                   ┌────────────┼────────────┐
                   ▼            ▼            ▼
             policy A      policy B      policy N
                   └────────────┼────────────┘
                                ▼
                      ┌──────────────────┐
                      │ Metrics/assertions│
                      └────────┬─────────┘
                               ▼
                  report / shadow diff / exporter
```

### Scheduler SDK

`pkg/scheduler` is the narrow public contract shared by replay and shadow mode. Requests contain scheduling metadata, never raw prompts. Cluster snapshots and endpoint observations carry monotonic versions. Successful decisions identify the evaluated snapshot and include at least one finite explanation factor.

Policy implementations must:

- be safe for concurrent use;
- honor context cancellation;
- treat request and snapshot inputs as immutable;
- select only eligible endpoints;
- avoid wall-clock reads and uncontrolled randomness during replay;
- use stable endpoint ordering for deterministic tie-breaking;
- return typed errors for invalid input and empty candidate sets.

Stateful policies receive a fresh instance for every replay. Their state transition order is determined solely by the event engine.

### Trace capture

The capture proxy will forward OpenAI-compatible streaming and non-streaming requests while asynchronously recording a bounded metadata envelope. Raw prompts and generated content are excluded by default. Prefix affinity is represented by a keyed fingerprint whose secret is supplied by the operator and is not stored in the trace.

Capture is fail-open for serving traffic: a full or unavailable trace sink increments a loss counter but does not delay the upstream request. Operators can choose a fail-closed mode only for controlled benchmark environments.

### Deterministic replay engine

Replay uses a virtual monotonic clock and a priority queue ordered by `(timestamp, event-kind-order, stable-sequence)`. It never sleeps and does not derive simulation behavior from goroutine scheduling. Each policy run receives an independent deep copy of mutable worker and policy state.

Randomized policies must use a recorded seed. Reports include the seed, trace digest, configuration digest, InferLab version, and model calibration version.

### Worker model

The initial worker model separates prefill and decode because they stress accelerators differently. It tracks queue state, active sequences, token budgets, continuous batching boundaries, cache residency, adapter residency, cancellation, and endpoint health. Calibration parameters are imported from real backends; every report discloses prediction error and avoids presenting simulated estimates as observed hardware results.

### Metrics and assertions

Reports distinguish observed, predicted, and derived values. Core dimensions include tenant, priority class, model, adapter, endpoint, and policy. High-cardinality request IDs remain in trace/report artifacts and are not metric labels.

Assertions are first-class replay outputs. Examples include maximum tenant concurrency, deadline attainment, starvation bounds, capacity-proportional dispatch, maximum queue residence, and fairness indices. A policy that improves aggregate P99 while violating a mandatory invariant is rejected.

### Shadow mode

Shadow evaluation receives an immutable copy of scheduling metadata after the production decision path. It may not alter headers, endpoint choice, request body, or backpressure. Bounded queues, deadlines, and circuit breakers isolate shadow failures from production.

Correlation records compare production and candidate choices using actual endpoint outcomes where available. Recommendations include confidence and sample sufficiency; they never claim causality from a simple decision mismatch.

### Production integrations

The primary integration target is the llm-d Router's Endpoint Picker plugin/configuration model. Adapters translate InferLab request/snapshot concepts into upstream contracts at the repository edge. The scheduler SDK does not import Kubernetes, Envoy, vLLM, SGLang, or llm-d packages, keeping the core deterministic and avoiding dependency lock-in.

Integration compatibility is tested against explicit upstream versions. Generated artifacts include that target version and fail validation when an unsupported schema is requested.

## Consistency and failure semantics

Version zero is reserved for unknown state. A decision must identify the exact cluster snapshot it evaluated. Replay rejects malformed state; shadow mode records and classifies it without impacting production.

Distributed state is an advanced, experimental layer. Its contract will define queue ownership, fencing tokens, idempotent admission, quota accounting, reconciliation, and explicit fail-open/fail-closed behavior before any Redis or etcd backend is selected.

## Compatibility policy

Before v1.0, public Go packages and trace schemas may change between minor releases, with migrations documented in release notes. From v1.0, the project will follow semantic versioning, support the current and previous trace schema, and publish a deprecation window for public contracts.

## Security and privacy boundary

The threat model includes prompt reconstruction, tenant identity leakage, malicious traces, decompression/size bombs, path traversal in report output, untrusted policy code, forged endpoint state, secrets in configuration, and shadow-mode resource exhaustion. Security work is scheduled before the first public release; see [SECURITY.md](../SECURITY.md) for reporting.

## Upstream references

- [llm-d Router architecture](https://github.com/llm-d/llm-d-router/blob/main/docs/architecture.md)
- [Gateway API Inference Extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension)
- [vLLM documentation](https://docs.vllm.ai/)
- [SGLang documentation](https://docs.sglang.ai/)
