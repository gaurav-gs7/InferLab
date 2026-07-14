# Design principles

## Evidence over demos

A feature is complete when its behavior is specified, tested, measured, and reproducible. A screenshot or happy-path demo is useful documentation, not validation.

## Determinism is an API guarantee

Given the same change, evidence, policy, seed, and InferLab version, normalization and evaluation must produce byte-equivalent canonical results. Any intentional source of nondeterminism is recorded in the safety case.

## Valid evidence before conclusions

Stale, incomplete, incompatible, ambiguous, or out-of-distribution evidence cannot support a release claim. Performance improvements never waive mandatory correctness, fairness, resilience, cost, or coverage gates.

## Privacy by construction

Raw prompts and responses are not captured by default. Sensitive metadata has explicit classification, retention, and redaction behavior. Capture failure cannot silently expose content or disrupt production traffic.

## Explain every release decision

Every PASS, BLOCK, and INCONCLUSIVE reason links to its policy, runtime identity, workload region, and evidence graph nodes. Explanations are stable enough for machine comparison and readable enough for release review.

## Calibrate claims

Simulation estimates are labelled predictions. Hardware results identify model and tokenizer revisions, engine and container, CUDA and driver, accelerator, scheduler/kernel configuration, workload, warm-up, sample size, and uncertainty. Prediction error and validity limits are published metrics.

## Reuse evidence producers

Benchmark tools, simulators, serving systems, and chaos executors remain independent upstreams. InferLab adds a versioned adapter when they already produce the required evidence and builds a new execution engine only for a documented gap that cannot be addressed upstream.

## Integrate at the edge

The core has no Kubernetes or serving-engine dependency. Upstream-specific adapters live at explicit boundaries and are compatibility-tested against pinned versions.

## Boring operations are a feature

Bounded memory, cancellation, timeouts, health endpoints, telemetry, safe defaults, graceful shutdown, schema migration, and actionable errors are part of the product—not release polish.

## Plans are not authority

A change budget or generated execution plan does not authorize cloud, quota, support, billing, cluster, deployment, or repository-identity mutations. Each external action requires explicit operator approval for that action.

## Honest project status

Documentation distinguishes implemented, experimental, and planned capabilities. Benchmarks and badges must be reproducible from the referenced revision.
