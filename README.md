# InferLab

**Uncertainty-aware pre-production safety evidence for LLM inference changes.**

[![CI](https://github.com/gaurav-gs7/InferLab/actions/workflows/ci.yml/badge.svg)](https://github.com/gaurav-gs7/InferLab/actions/workflows/ci.yml)
[![CodeQL](https://github.com/gaurav-gs7/InferLab/actions/workflows/codeql.yml/badge.svg)](https://github.com/gaurav-gs7/InferLab/actions/workflows/codeql.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

> **Status:** pre-alpha. The immutable inference-change contract, scheduler SDK, deterministic round-robin baseline, and versioned privacy-safe trace codec are implemented. Simulation, sparse GPU calibration, fault campaigns, counterexample search, and signed safety cases are active milestones—not shipped claims.

InferLab helps AI infrastructure teams answer a risky question before changing a model, engine, batching policy, replica count, or accelerator configuration:

> Is this inference change safe enough to deploy, where does the evidence stop, and what is the cheapest next experiment that would reduce that uncertainty?

InferLab is designed to combine deterministic trace replay, a calibrated performance model, a small number of real GPU measurements, bounded fault campaigns, and explicit SLO/cost/fairness policies. Its end product is a reproducible safety case with one of three decisions: `PASS`, `BLOCK`, or `INCONCLUSIVE`. The third result is deliberate: insufficient or out-of-distribution evidence must not be presented as confidence.

## Why InferLab exists

Production systems such as [vLLM](https://github.com/vllm-project/vllm), [SGLang](https://github.com/sgl-project/sglang), the [llm-d Router](https://github.com/llm-d/llm-d-router), and the [Kubernetes Gateway API Inference Extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension) already serve and route inference traffic. InferLab is not another model server, gateway, or benchmark leaderboard. It is a pre-production evidence layer around serving changes.

| Existing serving layer | InferLab evidence layer |
| --- | --- |
| Executes live inference | Replays a fixed, privacy-safe workload before deployment |
| Reports measurements for one configuration | Models a candidate and publishes calibration error |
| Surfaces operational metrics | Evaluates machine-readable latency, fairness, cost, and risk policies |
| Fails under real incidents | Searches bounded replica-loss and long-context fault spaces |
| Deploys a chosen configuration | Produces reviewable evidence tied to immutable inputs |

The v0.1 support envelope is intentionally small: single-node vLLM on an NVIDIA L4 (`g6.xlarge`), immutable model/container revisions, continuous-batching changes, multi-tenant trace replay, and two inference-specific fault families. A narrow system that can quantify its error is more useful than a broad compatibility matrix built on unvalidated assumptions.

## Target workflow

```text
immutable change + privacy-safe trace + deployment policies
                           │
                           ▼
              calibrated replay and uncertainty
               ┌───────────┴───────────┐
               ▼                       ▼
        bounded fault search     sparse GPU probes
               └───────────┬───────────┘
                           ▼
           counterexamples + evidence manifest
                           │
                           ▼
                 PASS / BLOCK / INCONCLUSIVE
```

## Inference-change contract

Every run starts from a strict, versioned JSON document. It pins baseline and candidate engine images by digest, model revisions, supported hardware, scheduler controls, trace location, tenant weights, policy thresholds, bounded faults, and hard cost/GPU-minute ceilings. The canonical SHA-256 digest identifies the change throughout its evidence chain.

```bash
go run ./cmd/inferlab change validate examples/qwen-vllm-batching-change.json
go run ./cmd/inferlab change digest examples/qwen-vllm-batching-change.json
```

See the [contract specification](docs/inference-change.md), [published JSON Schema](schemas/change/v1/inference-change.schema.json), and [complete example](examples/qwen-vllm-batching-change.json).

## Scheduler SDK

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

## Privacy-safe traces

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
./bin/inferlab change validate examples/qwen-vllm-batching-change.json
```

The CLI currently validates and identifies an experiment definition. It does not yet claim to execute the experiment; replay and evidence commands will land only with their corresponding engines and acceptance tests.

## Engineering standards

InferLab treats reproducibility and correctness as product features:

- deterministic scheduling inputs and stable tie-breaking;
- immutable change inputs and content-addressed evidence;
- mandatory explanations for every endpoint selection;
- privacy-safe capture with no raw prompts by default;
- unit, race, fuzz, golden, integration, and failure-injection tests;
- explicit fairness and SLO assertions, not dashboard-only evaluation;
- calibrated simulation with prediction error published beside results;
- explicit `INCONCLUSIVE` results for insufficient or out-of-distribution evidence;
- budget ceilings and teardown verification for cloud experiments;
- versioned schemas and compatibility policy;
- least-privilege CI, vulnerability scanning, and supply-chain provenance;
- honest benchmarks with committed workloads, configuration, and methodology.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the local merge gate and review expectations.

## Project documentation

- [Architecture and component boundaries](docs/architecture.md)
- [Inference-change contract](docs/inference-change.md)
- [Trace format and privacy contract](docs/trace-format.md)
- [Design principles](docs/design-principles.md)
- [Security policy](SECURITY.md)
- [Governance](GOVERNANCE.md)
- [Support](SUPPORT.md)

## Contributing

Issues and pull requests are welcome. For substantial changes, open a design issue before implementation so contracts, benchmarks, and integration assumptions can be reviewed together. All participants must follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

Apache License 2.0. See [LICENSE](LICENSE).
