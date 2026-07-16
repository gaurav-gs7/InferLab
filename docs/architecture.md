# Architecture

## Product boundary

InferLab is an uncertainty-aware release-assurance plane for LLM inference infrastructure changes. It does not serve prompts, generate load, emulate vLLM, operate a cluster, or own benchmark/simulator fidelity. It consumes evidence produced by those systems and answers a narrower question:

> Is the available evidence valid, compatible, sufficient, and policy-compliant for this exact proposed change?

Its responsibility begins with a validated inference-change document and evidence from one or more producers. It ends with a content-addressed safety case:

1. identify the baseline, candidate, workload, policies, faults, and budget;
2. normalize producer output without erasing source semantics or provenance;
3. bind every evidence item to an exact observed runtime signature;
4. exclude stale, incompatible, incomplete, or ambiguous evidence;
5. calculate workload coverage, drift, uncertainty, and sample sufficiency;
6. evaluate mandatory latency, fairness, resilience, and cost policies;
7. minimize and verify policy-breaking counterexamples;
8. return `PASS`, `BLOCK`, or `INCONCLUSIVE`; and
9. package claims, limitations, evidence, and artifact digests into a signed safety case.

The initial contract envelope is deliberately narrow: vLLM on an NVIDIA L4 (`g6.xlarge`), immutable model/container revisions, continuous-batching changes, up to two replicas, and bounded replica-loss plus long-context faults. This is a validation envelope, not a claim that InferLab must execute or simulate vLLM itself.

## Evidence flow

```text
                   ┌────────────────────┐
                   │ InferenceChange v1 │
                   │ intent + policies  │
                   └─────────┬──────────┘
                             │
       ┌─────────────────────┼──────────────────────┐
       ▼                     ▼                      ▼
┌─────────────┐       ┌──────────────┐       ┌──────────────┐
│  GuideLLM   │       │inference-perf│       │ simulators / │
│ observations│       │ observations │       │ fault tools  │
└──────┬──────┘       └──────┬───────┘       └──────┬───────┘
       └─────────────────────┼──────────────────────┘
                             ▼
                    ┌──────────────────┐
                    │ Adapter boundary │
                    │ source semantics │
                    └────────┬─────────┘
                             ▼
              ┌────────────────────────────┐
              │ Evidence normalizer/index  │
              │ provenance + runtime IDs   │
              └──────────────┬─────────────┘
                             ▼
              ┌────────────────────────────┐
              │ Validity / drift / coverage│
              │ uncertainty / sufficiency │
              └──────────────┬─────────────┘
                             ▼
              ┌────────────────────────────┐
              │ Policy + counterexamples   │
              └──────────────┬─────────────┘
                             ▼
              ┌────────────────────────────┐
              │ Signed inference safety case│
              │ PASS/BLOCK/INCONCLUSIVE    │
              └────────────────────────────┘
```

## Core contracts

### Change specification

`pkg/change` is the root trust boundary for experiment intent. The strict decoder bounds input size, rejects unknown fields and trailing values, and validates the supported envelope. Images require immutable SHA-256 digests; model revisions must be immutable hexadecimal identities; trace URIs may not contain credentials. Canonical JSON normalizes semantically unordered collections before producing the change digest.

The change digest identifies intent, not execution. It cannot establish what software or hardware actually ran and cannot make an evidence item valid by itself.

### Runtime signature

`pkg/evidence` implements the runtime signature, compatibility results, and canonical digests. Every evidence-producing attempt must report observed identity for all material dimensions:

- model weights and tokenizer revisions;
- quantization method and material quantization configuration;
- inference-engine revision and immutable container digest;
- CUDA/runtime, driver, and GPU SKU;
- replica/topology and scheduler configuration; and
- kernels or feature flags known to alter performance semantics.

Declared intent, observed identity, and unknown identity are different states. A declared value is never copied into the observed signature merely because a producer omitted it. Unknown material identity prevents exact compatibility and normally contributes to `INCONCLUSIVE`.

Compatibility is a versioned policy result: `exact`, `compatible-by-policy`, `incompatible`, or `unknown`. The default is exact matching. Any weaker rule must name the ignored dimension, rationale, evidence, rule version, and affected claim types. Material changes invalidate dependent evidence transitively.

### Evidence envelope

The bounded, strict `pkg/evidence.Envelope` classifies values as:

- `observed`: directly measured from an identified runtime;
- `predicted`: emitted by a simulator or analytical model;
- `derived`: calculated from identified parent evidence; or
- `asserted`: supplied metadata not independently observed.

Each envelope binds source tool/revision, adapter revision, runtime signature, workload signature, attempt, timestamps, units, completeness, raw artifact digests, and transformation lineage. Normalization preserves the source value and definition. Similar labels such as TPOT, ITL, normalized TPOT, or goodput are not pooled until a versioned semantic rule proves compatibility.

Partial output remains partial. Adapter crashes, missing samples, non-finite values, ambiguous units, conflicting identities, and digest mismatches fail closed for the affected claims.

## Evidence producers and adapters

### Adapter boundary

Third-party tools remain out of process behind a bounded, versioned protocol. An adapter declares supported producer versions, report schemas, capabilities, metric mappings, and known losses. It supports cancellation and deterministic normalization; it does not reinterpret unknown fields optimistically.

`pkg/adapter` implements this protocol, lossless mapping records, process cancellation/output bounds, and pinned GuideLLM and inference-perf conformance projections. Those projections are deliberately narrower than full native upstream report support. Simulator adapters may target Vidur, LLMServingSim, Doubleword Inference Lab, InferSim, or another system only after a pinned public fixture passes the same conformance suite.

### Simulator role

Simulation is an evidence class, not the architecture center. InferLab does not need a built-in vLLM facsimile to evaluate simulator output. A tiny analytical fixture source may exercise CI contracts, but it must not be presented as a performance model.

Predictions always retain their producer identity, calibration relationship, error, and validity envelope. Predicted evidence cannot silently replace an observation required by policy. Out-of-distribution prediction yields a coverage gap and contributes to `INCONCLUSIVE`.

### Trace boundary

The v1 trace codec records privacy-safe scheduling metadata with explicit units and strict limits. Tenant identities and prefix tokens are protected with domain-separated HMAC-SHA256. Raw prompt and response content are outside the default trust boundary. High-cardinality request identifiers remain in artifacts, never monitoring labels.

Traces can describe a workload for benchmark, simulator, and counterexample evidence. Capture is not required for v0.1; sanitized or synthetic inputs use the same digest and provenance rules. A future production capture adapter requires a separate threat model.

### Scheduler foundation

`pkg/scheduler` predates the release-assurance pivot. It remains a narrow deterministic contract for fixture policies and future routing-policy evidence. It does not justify building a complete scheduler suite or vLLM execution facsimile. Any retained scheduler work must directly support an evidence, fault, or policy-gate use case.

## Validity and uncertainty

### Evidence graph

The evidence index is a directed acyclic graph connecting intent, runtime, workload, producer, raw artifact, transformation, policy, counterexample, and claim nodes. A claim is admissible only when every required dependency is present, compatible, complete, and within its validity window.

`pkg/evidence.ValidityGraph` is an append-only dependency DAG. Invalidation propagates breadth-first, and reviewers receive a deterministic shortest machine-readable path explaining why an item was excluded—for example, candidate container changed → runtime signature incompatible → calibration stale → predicted TTFT unsupported → policy result inconclusive.

### Drift and coverage

Calibration compares predicted and observed evidence only inside compatible runtime and workload regions. Residuals, calibration age, workload drift, and producer drift are separate signals. The system does not claim universal simulator accuracy.

Coverage is measured across required concurrency, arrival rate, prompt/output length, tenant mix, request class, scheduler controls, and fault axes. Average-case coverage cannot conceal missing tails or noisy-neighbour regions. A policy may require observed evidence for selected regions even when predictions exist.

### Sample sufficiency

Evaluators use declared estimators, confidence bounds, and minimum sample rules. Measurement uncertainty, sampling uncertainty, model uncertainty, and adapter-mapping uncertainty remain distinct. Optional stopping, incompatible pooling, or missing tail samples are evidence limitations, not successful results.

`pkg/gate` implements the v1 observed-evidence admission path, workload-region coverage, 95% continuous/binomial confidence bounds, violation-risk comparison, fixed metric contracts, decision precedence, and the evidence-to-decision graph. Its result binds the canonical evaluation digest, so changing workload coordinates or uncertainty declarations changes the decision lineage.

## Faults and counterexamples

Fault contracts describe inference-infrastructure behavior rather than generic pod deletion or model-weight corruption. The vocabulary includes worker loss, Spot interruption, model-load failure, CUDA OOM, straggler GPU, KV-cache eviction storm, router partition, cancellation storm, telemetry loss, and stale performance profiles.

Only faults with implemented executors or conforming external evidence adapters are supported. The current gate admits bounded replica-loss and long-context-pressure evidence; it does not execute either fault. Each eventual executor must bind scope, trigger, seed, duration, recovery predicate, and blast radius. A campaign cannot target undeclared resources or exceed declared attempt/time/cost bounds.

A counterexample is the smallest known reproducible input region where a mandatory policy fails. `gate.Minimize` deterministically reduces request subsets, arrivals, concurrency, token shapes, and bounded fault parameters behind a caller-provided oracle. It records each accepted digest transition, reports budget-limited search honestly, never claims global minimality, and refuses a final candidate that does not pass the declared re-verification runs.

## Policy decisions

Core policies cover time to first token, time per output token/inter-token latency, SLO goodput, weighted multi-tenant fairness, noisy-neighbour isolation, recovery behavior, cost per successful token, and maximum violation probability.

Decision semantics are explicit:

- `PASS`: every mandatory policy holds, required evidence classes are present, coverage is adequate, and uncertainty is within tolerance over the declared envelope;
- `BLOCK`: adequate compatible evidence supports at least one reproducible mandatory-policy violation;
- `INCONCLUSIVE`: identity, coverage, calibration, sample size, provenance, metric semantics, or runner integrity is insufficient for either conclusion.

Invalid input and failed execution are not synonyms for `BLOCK`. They are reported separately and normally prevent a decision.

## Safety case

The safety case is a machine-readable evidence graph plus a human-readable summary. It contains immutable input digests, admitted and excluded evidence, runtime compatibility, coverage, uncertainty, policy outcomes, minimized counterexamples, actual cost, limitations, and cleanup status where external resources were used.

`pkg/safetycase` implements the bounded assembly descriptor, canonical manifest, raw-artifact closure, deterministic gate replay, exact claim/gap projection, detached Ed25519 signatures, and offline verification. A `BLOCK` case requires a valid counterexample artifact. Signing authenticates the root manifest and artifact closure. It does not make a prediction observed, make stale evidence current, identify an authorized organizational approver, or guarantee production safety.

## External execution and authorization

Core evaluation is local and performs no cloud, cluster, quota, support, billing, repository-identity, or deployment mutation. Planning may inspect supplied metadata or produce an execution plan but cannot treat a budget field as authority to act.

Every external mutation requires explicit operator approval for that specific action. Existing quota is an input; requesting quota is a separate administrative action. Provisioning, readiness, evidence upload, teardown, and cleanup verification form one idempotent state machine only after authorization. A run is incomplete until residual billable resources are checked.

## Compatibility policy

Before v1.0, public Go packages and JSON schemas may change between minor releases with migrations documented in release notes. Once an evidence schema is consumed by a release, its meaning is immutable: corrections require a new version rather than reinterpretation. Producer support is a matrix of exact tool/report versions backed by fixtures, not an unqualified product name.

## Security and privacy boundary

The threat model includes prompt reconstruction, tenant leakage, malicious documents and traces, size/decompression bombs, path traversal, forged measurements, artifact substitution, mutable references, compromised adapters/workers, leaked credentials, signature confusion, budget exhaustion, and incomplete teardown. Secrets are runtime references, never change or evidence fields. See [SECURITY.md](../SECURITY.md) for reporting.

## Non-goals for v0.1

- building a general-purpose LLM inference simulator;
- cloning vLLM queueing, batching, KV-cache, or execution behavior;
- competing with GuideLLM or inference-perf on load generation and visualization;
- replacing serving engines, routers, Kubernetes controllers, or chaos platforms;
- claiming support for a producer without pinned conformance fixtures; or
- automatically deploying, approving, or mutating external infrastructure.

See the [competitive landscape](landscape.md) for the projects that motivated these boundaries.
