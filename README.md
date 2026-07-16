# InferLab

**Uncertainty-aware pre-production safety evidence for LLM inference changes.**

[![CI](https://github.com/gaurav-gs7/InferLab/actions/workflows/ci.yml/badge.svg)](https://github.com/gaurav-gs7/InferLab/actions/workflows/ci.yml)
[![CodeQL](https://github.com/gaurav-gs7/InferLab/actions/workflows/codeql.yml/badge.svg)](https://github.com/gaurav-gs7/InferLab/actions/workflows/codeql.yml)
[![Safety gate proof](https://github.com/gaurav-gs7/InferLab/actions/workflows/safety-gate.yml/badge.svg)](https://github.com/gaurav-gs7/InferLab/actions/workflows/safety-gate.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

> **Status:** pre-alpha. Immutable inference-change intent, exact runtime/evidence validity, lossless adapter normalization, fail-closed uncertainty gating, reverified counterexample minimization, and signed offline-verifiable safety cases are implemented. Automated fault execution, real target-system validation, organizational signing policy, and a production `PASS` proof are not shipped claims.

> **Naming:** `InferLab` is a working repository name. It conflicts with Doubleword's existing [Inference Lab](https://github.com/doublewordai/inference-lab) simulator and must be replaced before v0.1. No rename will be performed without an explicit maintainer decision.

InferLab helps AI infrastructure teams answer a risky question before changing a model, engine, batching policy, replica count, or accelerator configuration:

> Is this inference change safe enough to deploy, where does the evidence stop, and what is the cheapest next experiment that would reduce that uncertainty?

InferLab is designed to consume evidence from real benchmark tools, simulators, and bounded fault experiments; reject stale or incompatible evidence; evaluate explicit SLO/cost/fairness policies; and minimize the smallest known policy-breaking workload. Its end product is a reproducible safety case with one of three decisions: `PASS`, `BLOCK`, or `INCONCLUSIVE`. The third result is deliberate: insufficient, stale, or out-of-distribution evidence must not be presented as confidence.

## Why InferLab exists

Production systems already serve and route inference traffic. [GuideLLM](https://github.com/vllm-project/guidellm) and [inference-perf](https://github.com/kubernetes-sigs/inference-perf) already generate serious endpoint benchmarks. [Vidur](https://github.com/microsoft/vidur), [LLMServingSim](https://github.com/casys-kaist/LLMServingSim), [Inference Lab](https://github.com/doublewordai/inference-lab), and [InferSim](https://github.com/alibaba/InferSim) already model inference systems. InferLab is not intended to replace any of them. It is the release-assurance plane that decides whether their evidence is valid and sufficient for a specific change.

| Evidence producer | InferLab release-assurance layer |
| --- | --- |
| Generates benchmark or simulation output | Normalizes source semantics without erasing provenance |
| Identifies one model/engine/hardware run | Invalidates reuse after a material runtime-signature change |
| Reports latency, throughput, or goodput | Evaluates machine-readable latency, fairness, cost, and risk policies |
| Shows that a policy failed | Minimizes and verifies the smallest known reproducing counterexample |
| Produces reports and charts | Emits a signed evidence graph and deterministic release decision |

The initial contract envelope remains intentionally small: single-node vLLM on an NVIDIA L4 (`g6.xlarge`), immutable model/container revisions, continuous-batching changes, multi-tenant workloads, and bounded replica-loss/long-context faults. Backend breadth is earned through versioned conformance fixtures, not claimed from file-format resemblance.

## Target workflow

```text
                         inference change
                                │
          ┌─────────────────────┼─────────────────────┐
          ▼                     ▼                     ▼
      GuideLLM            inference-perf       simulator/fault
     observations          observations            evidence
          └─────────────────────┼─────────────────────┘
                                ▼
                  evidence normalization + provenance
                                ▼
              runtime validity + coverage + uncertainty
                                ▼
              policy gate + counterexample minimization
                                ▼
                     PASS / BLOCK / INCONCLUSIVE
                                ▼
                       signed inference safety case
```

## Ownership boundary

The trusted core is limited to:

- exact runtime signatures spanning model, tokenizer, engine, container, CUDA, driver, GPU, scheduler, and material kernel configuration;
- source-neutral evidence envelopes that keep observed, predicted, and derived values distinct;
- transitive invalidation, drift, coverage, and sample-sufficiency analysis;
- inference-infrastructure fault semantics and bounded campaign evidence;
- deterministic minimization and verification of SLO-breaking workloads; and
- policy decisions and signed evidence graphs suitable for CI and deployment review.

Load generation, high-fidelity simulation, model serving, routing, and generic chaos execution are integration boundaries—not product claims. See the [competitive landscape](docs/landscape.md) for the evidence behind this decision.

## Inference-change contract

Every run starts from a strict, versioned JSON document. It pins baseline and candidate engine images by digest, model revisions, supported hardware, scheduler controls, trace location, tenant weights, policy thresholds, bounded faults, and hard cost/GPU-minute ceilings. The canonical SHA-256 digest identifies the change throughout its evidence chain.

```bash
go run ./cmd/inferlab change validate examples/qwen-vllm-batching-change.json
go run ./cmd/inferlab change digest examples/qwen-vllm-batching-change.json
```

See the [contract specification](docs/inference-change.md), [published JSON Schema](schemas/change/v1/inference-change.schema.json), and [complete example](examples/qwen-vllm-batching-change.json).

## Runtime identity and evidence validity

Evidence is reusable only when its material runtime identity is known and compatible. The v1 signature covers model and tokenizer revisions, quantization configuration, engine revision, container digest, CUDA, driver, GPU shape, topology, scheduler configuration, and material kernels. Missing values are `unknown`, never wildcards.

Compatibility produces one of four results: `exact`, `compatible-by-policy`, `incompatible`, or `unknown`. A named, versioned policy may ignore a known difference, but it cannot waive missing identity. The evidence envelope separately labels observations, predictions, derived metrics, and assertions; binds them to their source, adapter, workload, attempt, timestamps, artifacts, and transformations; and rejects complete evidence with unknown runtime dimensions.

```bash
go run ./cmd/inferlab runtime validate examples/runtime-signature-l4-vllm.json
go run ./cmd/inferlab evidence validate examples/guidellm-observed-evidence.json
```

See the [runtime and evidence contract](docs/evidence.md), [runtime schema](schemas/evidence/v1/runtime-signature.schema.json), and [evidence schema](schemas/evidence/v1/envelope.schema.json).

## Evidence adapters and lossless normalization

The adapter boundary pins the producer tool, version, report schema, adapter revision, source metric definition, and source unit before conversion. Original values and definitions remain beside explicit mapping records; the normalized evidence envelope is cryptographically bound to the exact raw report bytes. GuideLLM TPOT and inference-perf ITL remain separate semantics even when both use milliseconds.

External adapters run without a shell behind a versioned capability/normalize protocol with bounded input and output, strict JSON, an empty-by-default environment, cancellation, structured failures, and core-side response validation. Built-in GuideLLM and inference-perf support is deliberately limited to committed conformance projections; it is not advertised as broad native report compatibility. A generic constructor enforces predicted-only analytical adapters.

```bash
go run ./cmd/inferlab adapter list
go run ./cmd/inferlab adapter capabilities guidellm-fixture-v1
go run ./cmd/inferlab adapter normalize guidellm-fixture-v1 examples/guidellm-adapter-input.json
```

See the [adapter protocol and normalization specification](docs/adapters.md), [protocol schemas](schemas/adapter/v1), and [public-safe conformance fixtures](pkg/adapter/testdata).

## Uncertainty gate and reproducible counterexamples

The release gate admits only fresh, complete, compatible observed evidence inside an exact workload/fault region. It enforces minimum samples, declared uncertainty, fixed metric semantics, conservative 95% bounds, and maximum violation probability across TTFT, TPOT/ITL, goodput, fairness, noisy-neighbour isolation, recovery, and cost. Any mandatory violation yields `BLOCK`; any evidence gap yields `INCONCLUSIVE`; only an entirely supported policy set yields `PASS`.

Results contain stable findings, coverage, admission decisions, and an evidence graph linked to content digests. The deterministic, budget-bounded minimizer removes requests and reduces concurrency, token shapes, arrival timing, and the supported replica-loss or long-context fault dimensions, then re-verifies the final counterexample. Budget exhaustion and failed reproduction remain explicit.

```bash
go run ./cmd/inferlab gate evaluation validate examples/missing-evidence-gate.json
go run ./cmd/inferlab gate evaluate examples/missing-evidence-gate.json # exits 4: INCONCLUSIVE
```

See the [gate and counterexample specification](docs/gate.md), [gate schemas](schemas/gate/v1), and deliberately incomplete [public example](examples/missing-evidence-gate.json).

## Signed offline safety cases

A safety case binds the exact evaluation, replayed result, raw evidence bytes, counterexample, claims, gaps, limitations, and artifact sizes/digests beneath one canonical manifest. Assembly and offline verification both replay the deterministic gate. Detached Ed25519 signatures use a domain separator and public-key digest; signing authenticates provenance but cannot improve evidence class or sufficiency.

Only canonical relative regular files beneath one case root are admitted. Traversal, symlinks, duplicate paths, oversized artifacts, missing evidence bytes, result drift, claim drift, signature tampering, and key mismatch fail closed.

```bash
make demo-safety-case
```

The demo reproduces a synthetic `BLOCK` and an intentionally evidence-free `INCONCLUSIVE`, signs both with an ephemeral development key, verifies their complete closures offline, and deletes the private key. The repository deliberately publishes no `PASS` fixture without adequate observed target-system evidence.

See the [safety-case contract and CLI](docs/safety-case.md), [published schemas](schemas/safety-case/v1), [threat model](docs/threat-model.md), and [release-readiness audit](docs/release-readiness.md).

## Implemented supporting foundations

### Scheduler SDK

Policies implement one small, concurrency-safe contract:

```go
type Scheduler interface {
    Name() string
    Select(context.Context, Request, ClusterSnapshot) (Decision, error)
}
```

Every successful decision is versioned and explainable. Core validation rejects stale-snapshot mismatches, non-finite scores, missing explanations, and endpoints that are unhealthy, draining, or model-incompatible.

```go
decision := scheduler.Decision{
    EndpointID:      "worker-7",
    Score:           0.82,
    SnapshotVersion: 42,
    Reasons: []scheduler.Reason{
        {
            Code:         "prefix_cache_affinity",
            Message:      "78% of prompt blocks are already resident",
            Contribution: 0.38,
        },
    },
}
```

See the [architecture](docs/architecture.md) for evidence provenance, determinism, uncertainty, and integration boundaries.

The SDK remains useful for deterministic fixtures and future routing-policy evidence, but building a complete vLLM scheduler facsimile is not on the v0.1 critical path.

### Privacy-safe traces

The v1 JSONL trace codec records scheduling metadata with explicit units and strict resource bounds. Tenant identities and prefix tokens are protected with domain-separated HMAC-SHA256, optional metadata is fail-closed behind an allowlist, and raw content-shaped fields are rejected during decoding.

```go
protector, err := trace.NewProtector(operatorKey)
if err != nil {
    // Handle invalid key material.
}

tenantID, err := protector.TenantID("payments-copilot")
if err != nil {
    // Handle invalid tenant metadata.
}

prefixID, err := protector.PrefixFingerprint("qwen-32b", tokenIDs)
if err != nil {
    // Handle invalid model or token metadata.
}
```

See the [trace format specification](docs/trace-format.md) for the schema, compatibility rules, limits, and privacy boundary.

## Quick start

Prerequisites: Go 1.26 or newer.

```bash
make check
make build
make demo-safety-case
./bin/inferlab change validate examples/qwen-vllm-batching-change.json
./bin/inferlab runtime validate examples/runtime-signature-l4-vllm.json
./bin/inferlab evidence validate examples/guidellm-observed-evidence.json
./bin/inferlab adapter normalize guidellm-fixture-v1 examples/guidellm-adapter-input.json
```

The CLI validates and identifies change, runtime, evidence, normalized-report, gate-evaluation, gate-result, and safety-case documents; normalizes pinned producer fixtures; emits canonical gate decisions with stable exit codes; and assembles, signs, and verifies artifact closures offline.

## Engineering standards

InferLab treats reproducibility and correctness as product features:

- immutable change inputs and content-addressed evidence;
- exact runtime identity and fail-closed evidence invalidation;
- source-neutral adapters with versioned conformance fixtures;
- privacy-safe capture with no raw prompts by default;
- unit, race, fuzz, golden, integration, and failure-injection tests;
- explicit fairness and SLO assertions, not dashboard-only evaluation;
- observed, predicted, and derived evidence kept distinct;
- calibrated predictions with error and validity envelopes published beside results;
- reproducible minimal counterexamples rather than percentile-only failures;
- explicit `INCONCLUSIVE` results for insufficient or out-of-distribution evidence;
- budget ceilings and teardown verification for cloud experiments;
- versioned schemas and compatibility policy;
- least-privilege CI, module verification, vulnerability scanning, CodeQL, and dependency update policy;
- honest benchmarks with committed workloads, configuration, and methodology.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the local merge gate and review expectations.

## Project documentation

- [Architecture and component boundaries](docs/architecture.md)
- [Competitive landscape and positioning](docs/landscape.md)
- [ADR 0001: own release assurance, not simulation](docs/decisions/0001-evidence-plane-not-simulator.md)
- [Inference-change contract](docs/inference-change.md)
- [Runtime identity and evidence contract](docs/evidence.md)
- [Adapter protocol and lossless normalization](docs/adapters.md)
- [Uncertainty gate and reproducible counterexamples](docs/gate.md)
- [Signed safety cases and offline verification](docs/safety-case.md)
- [Threat model](docs/threat-model.md)
- [Release-readiness audit](docs/release-readiness.md)
- [Trace format and privacy contract](docs/trace-format.md)
- [Design principles](docs/design-principles.md)
- [Security policy](SECURITY.md)
- [Governance](GOVERNANCE.md)
- [Support](SUPPORT.md)

## Contributing

Issues and pull requests are welcome. For substantial changes, open a design issue before implementation so contracts, benchmarks, and integration assumptions can be reviewed together. All participants must follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

Apache License 2.0. See [LICENSE](LICENSE).
