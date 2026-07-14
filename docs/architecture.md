# Architecture

## Product boundary

InferLab is an uncertainty-aware pre-production safety system for LLM inference changes. It does not serve prompts, train models, deploy candidates, or replace vLLM, SGLang, Kubernetes, or an inference gateway. It turns an immutable change proposal and a privacy-safe workload into reviewable evidence about latency, fairness, resilience, and cost.

Its responsibility begins with a validated inference-change document and ends with a content-addressed safety case:

1. validate and hash immutable experiment intent;
2. normalize privacy-safe workload observations into a versioned trace;
3. collect a small, actively selected set of real GPU measurements;
4. calibrate a deterministic performance model and publish its error;
5. replay baseline and candidate configurations on identical arrivals;
6. search bounded fault and workload spaces for counterexamples;
7. evaluate machine-readable latency, fairness, cost, and risk policies;
8. return `PASS`, `BLOCK`, or `INCONCLUSIVE` with provenance; and
9. tear down ephemeral evidence infrastructure and verify zero residual compute.

The initial support envelope is deliberately narrow: vLLM on an NVIDIA L4 (`g6.xlarge`), immutable model/container revisions, continuous-batching changes, up to two replicas, and replica-loss plus long-context-spike faults.

## Evidence flow

```text
┌────────────────────┐   ┌────────────────────┐   ┌──────────────────┐
│ InferenceChange v1 │   │ Privacy-safe trace │   │ Policy thresholds│
│ immutable + hashed │   │ bounded + versioned│   │ latency/cost/risk│
└──────────┬─────────┘   └──────────┬─────────┘   └────────┬─────────┘
           └────────────────────────┼──────────────────────┘
                                    ▼
                         ┌─────────────────────┐
                         │ Experiment planner  │
                         │ budget + coverage   │
                         └──────┬────────┬─────┘
                                │        │
                    ┌───────────┘        └───────────┐
                    ▼                                ▼
          ┌──────────────────┐             ┌──────────────────┐
          │ Sparse GPU probes│             │ Deterministic sim│
          │ observed evidence│────────────▶│ calibrated model │
          └──────────────────┘             └────────┬─────────┘
                                                    ▼
                                          ┌──────────────────┐
                                          │ Fault/counterex. │
                                          │ bounded search   │
                                          └────────┬─────────┘
                                                   ▼
                                          ┌──────────────────┐
                                          │ Signed safety case│
                                          │ P/B/INCONCLUSIVE │
                                          └──────────────────┘
```

### Change contract

`pkg/change` is the root trust boundary for experiment intent. The strict decoder bounds input size, rejects unknown fields and trailing values, and validates the supported envelope. Images require immutable SHA-256 digests; common mutable model aliases are forbidden; trace URIs may not contain credentials. Canonical JSON normalizes semantically unordered collections before producing the change digest.

The digest is necessary but not sufficient provenance. A safety case must also bind the trace digest, InferLab revision, runner image, calibration dataset, random seeds, cloud resource identity, timestamps, raw output digests, and policy evaluator version.

### Trace boundary

The v1 trace codec records scheduling metadata with explicit units and strict limits. Tenant identities and prefix tokens are protected with domain-separated HMAC-SHA256. Raw prompt and response content are outside the default trust boundary. High-cardinality request identifiers remain in artifacts, never monitoring labels.

Trace capture is not required for the first end-to-end public proof: sanitized and synthetic traces can exercise the same contract. A future capture adapter must be fail-open for serving traffic, bounded, and independently threat-modeled before it can observe production requests.

### Experiment planner

The planner treats cloud credits as a finite evidence budget. It chooses calibration points for expected information gain, enforces the change document's dollar and GPU-minute ceilings, and refuses a run when account state, quota, region capacity, or model licensing cannot satisfy the declared plan.

Cloud workers are ephemeral. Provisioning, readiness, artifact collection, and teardown are one state machine. A successful experiment is not complete until instance termination, storage cleanup, and billing-resource verification are recorded. Interrupted runs use the same idempotent cleanup path.

### Observed evidence and calibration

Observed hardware measurements and simulator predictions are different evidence classes. Raw measurements record warm-up policy, repetition count, engine/model/hardware identity, token-shape distribution, batching controls, and measurement uncertainty. The profiler derives prefill, decode, memory, and batching response surfaces without rewriting observations.

Calibration publishes holdout error and a validity envelope. A prediction outside that envelope is out-of-distribution and contributes to `INCONCLUSIVE`, not extrapolated certainty. Active calibration may request another real measurement only when its expected uncertainty reduction justifies the remaining budget.

### Deterministic replay

Replay uses a virtual monotonic clock and a priority queue ordered by `(timestamp, event-kind-order, stable-sequence)`. It never sleeps or derives results from goroutine scheduling. Baseline and candidate runs receive independent copies of mutable worker and scheduler state, the same workload, and recorded random seeds.

The worker model separates prefill and decode, tracks queue state, active sequences, token budgets, continuous-batching boundaries, cache residency, cancellation, and endpoint health. Every simulated metric is labelled predicted and linked to a calibration version.

### Scheduler SDK

`pkg/scheduler` is a narrow deterministic contract used by replay. Requests contain scheduling metadata, never raw prompts. Cluster observations carry monotonic versions. A successful decision identifies its evaluated snapshot and includes finite explanation factors.

Policies must be concurrency-safe, honor cancellation, treat inputs as immutable, select only eligible endpoints, avoid uncontrolled wall-clock reads and randomness, use stable tie-breaking, and return typed errors for invalid input or empty candidate sets. Stateful policies receive a fresh instance for every replay.

### Faults and counterexamples

Fault campaigns are bounded by the change contract. The initial families are replica loss across declared durations/probabilities and long-context spikes across declared token counts. Search may refine within those bounds but cannot silently invent a stronger adversary or exceed the experiment budget.

A counterexample is a reproducible input region where a mandatory policy fails. It records the minimal known trigger, search bounds, seed, metric evidence, and whether the result was observed or predicted. Aggregate improvement never overrides a mandatory violation.

### Policy evaluation and uncertainty

Core policies cover time to first token, time per output token, weighted multi-tenant fairness, cost per million tokens, and maximum violation probability. Evaluators operate on distributions and confidence bounds rather than dashboard point estimates.

Decision semantics are explicit:

- `PASS`: all mandatory policies hold and uncertainty is below the declared tolerance over the required envelope;
- `BLOCK`: at least one reproducible mandatory-policy violation is supported by adequate evidence;
- `INCONCLUSIVE`: coverage, calibration, sample size, provenance, or infrastructure integrity is insufficient for either conclusion.

### Safety case

The safety case is a machine-readable manifest plus a human-readable summary. It contains immutable input digests, evidence classification, calibration/coverage results, policy outcomes, counterexamples, actual cost, limitations, and teardown proof. Signing authenticates the manifest and artifact digests; it does not transform predicted evidence into observed evidence or guarantee production safety.

## Consistency and failure semantics

Version zero is reserved for unknown scheduler state. Replay rejects malformed state. Evidence ingestion is fail-closed: partial uploads, missing digests, mismatched identities, non-finite metrics, and ambiguous units invalidate the affected evidence.

Runner failures do not become policy failures. They are classified separately and generally yield `INCONCLUSIVE`. Retrying preserves experiment identity while recording an attempt number; observations from incompatible configurations may not be pooled.

## Compatibility policy

Before v1.0, public Go packages and JSON schemas may change between minor releases with migrations documented in release notes. Once evidence schemas are consumed by a released version, their meaning is immutable: corrections require a new version rather than reinterpretation. From v1.0, InferLab will follow semantic versioning and publish explicit support/deprecation windows.

## Security and privacy boundary

The threat model includes prompt reconstruction, tenant leakage, malicious documents and traces, size/decompression bombs, path traversal, forged measurements, artifact substitution, mutable model/container references, compromised cloud workers, leaked credentials, budget exhaustion, and incomplete teardown. Secrets are references supplied at runtime, never contract fields or evidence content. See [SECURITY.md](../SECURITY.md) for vulnerability reporting.

## Planned integrations

The first real backend is vLLM on one L4 GPU. SGLang, llm-d, Kubernetes inference routing, capture proxies, shadow mode, and production exporters remain post-v0.1 integrations. New integrations enter at repository edges and must not weaken deterministic core contracts or mix observed and predicted evidence.
