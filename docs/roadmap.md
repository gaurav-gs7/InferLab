# 35-day delivery roadmap

This roadmap is ordered by technical dependency and risk, not visual appeal. Each day ends with one reviewable, tested commit pushed only after its acceptance criteria pass. A day may take more than one calendar day when correctness demands it; scope is never marked complete to preserve a schedule.

## Daily delivery protocol

Every daily milestone follows the same merge gate:

1. sync with `main` and confirm a clean worktree;
2. implement only the day's declared scope;
3. add unit, race, fuzz, golden, integration, or failure tests in proportion to risk;
4. update user and contributor documentation;
5. run formatting, vet, unit tests, race tests, and relevant benchmarks;
6. inspect the complete diff and record limitations;
7. create a conventional commit and push it to GitHub;
8. verify required GitHub checks before calling the day complete.

## Week 1 — trustworthy foundation

### Day 1: contracts and repository quality

- Define request, versioned cluster snapshot, endpoint, decision, and reason contracts.
- Enforce validation and endpoint eligibility invariants.
- Implement a concurrency-safe deterministic round-robin baseline.
- Establish CLI skeleton, unit/race tests, CI, CodeQL, OSS governance, and architecture docs.
- **Acceptance:** `make check` passes; invalid decisions cannot cross the SDK boundary; repository status is honest and contribution-ready.

### Day 2: versioned privacy-safe trace schema

- Specify JSONL envelope, schema version, units, field presence, and forward-compatibility rules.
- Implement streaming encoder/decoder with byte, line, token, and record limits.
- Add keyed prefix fingerprints, tenant pseudonymization, configurable metadata allowlist, and deterministic canonicalization.
- Add malicious-input, round-trip, golden, and fuzz tests.
- **Acceptance:** no raw content field exists in the default schema; corrupted and oversized traces fail with record location and typed errors.

### Day 3: transparent capture proxy

- Forward OpenAI-compatible chat/completions requests, including SSE streaming and cancellation.
- Capture request/response timing and usage metadata without buffering full streams.
- Add bounded asynchronous trace sink, backpressure policy, loss metrics, timeouts, graceful shutdown, and redaction audit logs.
- **Acceptance:** byte/stream semantics match upstream in integration tests; sink failure does not affect serving in default fail-open mode.

### Day 4: deterministic discrete-event kernel

- Implement virtual clock, stable event heap, cancellation, and same-timestamp ordering rules.
- Record seeds and canonical run manifests.
- Add determinism golden tests across repeated and parallel test execution.
- **Acceptance:** identical inputs produce byte-identical canonical event output without wall-clock sleeps.

### Day 5: virtual worker lifecycle

- Model queue admission, prefill, decode, continuous batching boundaries, completion, cancellation, and endpoint failure.
- Define explicit state machine transitions and illegal-state diagnostics.
- Add property and invariant tests for token conservation and terminal requests.
- **Acceptance:** every accepted request reaches exactly one terminal state and no worker exceeds configured capacity.

## Week 2 — credible replay and comparison

### Day 6: baseline scheduling suite

- Add least-active, least-queued-work, and seeded-random policies beside round-robin.
- Standardize structured reason codes and score normalization.
- Add policy registry and duplicate-name/config validation.
- **Acceptance:** all baselines pass the same contract suite and stable tie-breaking tests.

### Day 7: calibrated latency model

- Separate prefill latency, decode time per output token, batching efficiency, and cache-hit effects.
- Add versioned calibration files and monotonic interpolation with out-of-range behavior.
- Include prediction intervals and calibration provenance.
- **Acceptance:** calibration fixtures reproduce held-out measurements within declared error bounds.

### Day 8: multi-policy replay CLI

- Implement `inferlab replay` with repeated scheduler flags, config loading, seed control, and bounded resource use.
- Isolate mutable state per policy and emit a canonical run manifest.
- Add progress, cancellation, structured errors, and atomic output writes.
- **Acceptance:** one trace replays against multiple policies reproducibly; interrupted runs leave no partial report presented as complete.

### Day 9: metrics and comparison report

- Compute TTFT/TPOT/latency percentiles, throughput, SLO attainment, cache affinity, queue residence, utilization, starvation, and normalized cost.
- Separate observed, simulated, and derived values.
- Export JSON and human-readable table formats with schema versions.
- **Acceptance:** metric implementations pass analytical fixtures and do not use unbounded high-cardinality labels.

### Day 10: correctness assertion engine

- Add typed assertions for SLOs, concurrency, quotas, queue age, starvation, distribution, and fairness.
- Report counterexamples with tenant/request/event context.
- Make failed mandatory assertions return a distinct CLI exit code.
- **Acceptance:** golden scenarios intentionally pass and fail every assertion type; aggregate latency gains cannot mask a mandatory failure.

## Week 3 — multi-tenant fairness specialization

### Day 11: hierarchical token admission

- Implement global, organization, tenant, model, and priority-class token buckets.
- Support reserved and borrowable capacity with deterministic replenishment.
- Explain admission, delay, and rejection decisions.
- **Acceptance:** property tests prove limits, borrowing repayment, and no token creation under concurrent arrivals.

### Day 12: deficit round robin

- Implement token-cost-aware DRR with configurable quanta and tenant weights.
- Define idle-credit caps and active-set lifecycle.
- Benchmark scheduler overhead across tenant cardinalities.
- **Acceptance:** long-run allocation converges within tolerance and large prompts cannot monopolize dispatch.

### Day 13: weighted fair queueing

- Implement virtual finish times using overflow-safe fixed-point arithmetic.
- Support hierarchical weights and deterministic ties.
- Compare approximation error and overhead against DRR.
- **Acceptance:** analytical traces match expected service order and weighted shares without floating-point drift.

### Day 14: deadline and locality composition

- Implement EDF and SLO slack scoring.
- Compose deadline urgency with prefix-cache and adapter locality under explicit weights/constraints.
- Surface predicted deadline breaches before dispatch.
- **Acceptance:** feasible deadlines are prioritized while locality optimization cannot violate hard urgency constraints.

### Day 15: bounded starvation and preemption

- Add aging, maximum queue-residence bounds, batch preemption semantics, and priority inheritance where blocking is modeled.
- Preserve or account for KV recomputation cost during preemption.
- Add adversarial sustained-priority workloads.
- **Acceptance:** starvation bounds hold under declared capacity assumptions and impossible guarantees produce explicit overload evidence.

## Week 4 — production-safe shadowing

### Day 16: operational telemetry

- Add structured logs, Prometheus metrics, OpenTelemetry traces, health/readiness endpoints, and run IDs.
- Define metric cardinality budgets and sensitive-field policy.
- Add graceful shutdown and configuration validation.
- **Acceptance:** telemetry tests prove no tenant/request identifiers enter metric labels and shutdown drains within its deadline.

### Day 17: isolated shadow tap

- Feed metadata to candidate schedulers through bounded queues with deadlines and circuit breakers.
- Guarantee no mutation of production requests or responses.
- Classify drops, stale state, timeouts, and policy failures.
- **Acceptance:** load and failure injection show production latency/availability are unchanged within the declared overhead budget.

### Day 18: production/candidate correlation

- Correlate production choice, candidate choice, snapshot version, queue prediction, and observed outcome.
- Handle missing, late, duplicated, and reordered observations idempotently.
- Define retention and deletion controls.
- **Acceptance:** correlation fixtures produce complete accounting with no false matches.

### Day 19: shadow evaluator and recommendations

- Compute decision agreement, opportunity cost, fairness deltas, oscillation, SLO deltas, and cache opportunities.
- Add confidence intervals, minimum sample thresholds, and reject/hold/advance rules.
- Generate evidence-linked reports rather than causal claims.
- **Acceptance:** insufficient or biased samples never produce an advance recommendation.

### Day 20: resilience and performance gate

- Add race, fuzz, soak, overload, cancellation-storm, slow-sink, corrupt-state, and endpoint-churn tests.
- Establish CPU, allocation, memory, and scheduling-latency budgets.
- Publish benchmark methodology and baseline artifacts.
- **Acceptance:** no leaks/races; bounded degradation and recovery meet documented budgets.

## Week 5 — real ecosystem integration

### Day 21: llm-d Router mapping

- Map InferLab filters, scorers, pickers, flow control, request metadata, and endpoint state to current Router contracts.
- Pin a supported Router release and add compatibility fixtures.
- Document intentional semantic gaps.
- **Acceptance:** mappings are versioned, loss is explicit, and incompatible targets fail closed during export.

### Day 22: Router configuration exporter

- Export validated `EndpointPickerConfig` plugin and scheduling-profile configuration.
- Preserve reason/weight provenance in an accompanying manifest.
- Validate generated YAML against upstream types or schemas.
- **Acceptance:** upstream validation accepts golden exports and round-trip semantic checks pass.

### Day 23: Kubernetes deployment package

- Add least-privilege manifests/Helm chart for capture and shadow components.
- Configure security contexts, resources, probes, disruption budgets, network policy, and service account boundaries.
- Add schema, render, and policy tests.
- **Acceptance:** manifests pass server-side dry-run against the supported Kubernetes matrix and security-policy checks.

### Day 24: Envoy/Router end-to-end test

- Stand up a local Gateway/Envoy, Router EPP, InferLab shadow component, and fake model workers.
- Exercise streaming, cancellation, endpoint churn, and overload.
- Store deterministic test evidence in CI artifacts.
- **Acceptance:** a kind-based test proves non-interference and end-to-end correlation.

### Day 25: serving-engine adapters

- Add vLLM and SGLang observation/trace converters with explicit version support.
- Normalize engine metrics without erasing source semantics.
- Add a lightweight real-backend validation recipe.
- **Acceptance:** captured observations replay successfully and unsupported/missing metrics are reported, never fabricated.

## Week 6 — distributed-state validation

### Day 26: shared-state protocol

- Specify queue ownership, fencing, idempotent admission, quota atomicity, cursor state, cache synchronization, and reconciliation.
- Implement a deterministic in-memory reference model.
- Define fail-open/fail-closed modes per operation.
- **Acceptance:** the protocol has executable invariants before an external datastore is introduced.

### Day 27: durable backend

- Implement one justified backend after protocol benchmarks (etcd or Redis), with leases, transactions, retries, and telemetry.
- Add schema/version migration and safe startup checks.
- **Acceptance:** backend behavior matches the reference model under normal operation.

### Day 28: distributed failure laboratory

- Inject partitions, stale reads, lease expiry, process loss, delayed events, clock skew, and duplicate delivery.
- Detect double admission, lost quota, split-brain dispatch, and reconciliation lag.
- Compare consistency/performance trade-offs.
- **Acceptance:** safety violations are either prevented or surfaced as failed invariants with reproducible seeds.

### Day 29: distributed scalability benchmark

- Measure decision latency, throughput, datastore load, convergence, and fairness across scheduler replicas and tenants.
- Publish saturation points and resource profiles.
- Add regression thresholds to scheduled CI where stable.
- **Acceptance:** claims include configuration, uncertainty, and failure behavior—not peak throughput alone.

### Day 30: recovery and operational runbooks

- Document backup/restore, rolling upgrades, schema migration, degraded operation, incident diagnosis, and rollback.
- Exercise each runbook in automated or recorded drills.
- **Acceptance:** a new operator can recover from the tested failures without undocumented state edits.

## Week 7 — public-release credibility

### Day 31: benchmark corpus and reproducibility

- Publish synthetic workload generators and redistributable representative traces.
- Version hardware/backend calibration and benchmark manifests.
- Add one-command local baseline reproduction.
- **Acceptance:** a clean environment reproduces published tables within documented tolerance.

### Day 32: supply-chain hardening

- Add reproducible release builds, SBOMs, provenance attestations, signatures, pinned CI actions, and dependency policy.
- Scan containers and Go dependencies; document vulnerability response SLAs.
- **Acceptance:** release artifacts are verifiable from source and critical findings block release.

### Day 33: threat-model and privacy review

- Complete data-flow threat model, metadata classification, retention/deletion, abuse cases, and untrusted-trace controls.
- Add security regression and resource-exhaustion tests.
- Review defaults for least data and least privilege.
- **Acceptance:** high-risk threats have tested mitigations or explicit release blockers.

### Day 34: upstream contribution package

- Identify one independently useful llm-d Router or Gateway contribution discovered through integration.
- Prepare a focused issue/design note, compatibility evidence, tests, and minimal PR.
- Document InferLab's integration without coupling upstream to this repository.
- **Acceptance:** contribution is maintainer-reviewable on its own merits; no vanity or mass-generated change.

### Day 35: v0.1 release candidate

- Freeze scope, run the complete clean-room test matrix, audit docs/examples, and resolve release blockers.
- Publish architecture decision records, benchmark report, demo recording/script, migration notes, and limitations.
- Tag and sign v0.1.0 only after CI and artifact verification.
- **Acceptance:** every README claim is traceable to code, tests, or reproducible evidence; known limitations are prominent.

## Post-v0.1 directions

Potential work includes LoRA placement studies, multi-pool and heterogeneous-accelerator objectives, prefill/decode disaggregation policies, active queue management, learned latency predictors with drift detection, and additional production exporters. These are deliberately outside v0.1 until the validation core is credible.
