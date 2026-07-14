# InferLab

**Deterministic development, replay, and shadow validation for production LLM inference schedulers.**

[![CI](https://github.com/gaurav-gs7/InferLab/actions/workflows/ci.yml/badge.svg)](https://github.com/gaurav-gs7/InferLab/actions/workflows/ci.yml)
[![CodeQL](https://github.com/gaurav-gs7/InferLab/actions/workflows/codeql.yml/badge.svg)](https://github.com/gaurav-gs7/InferLab/actions/workflows/codeql.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

> **Status:** pre-alpha. The scheduler SDK, deterministic round-robin baseline, and versioned privacy-safe trace codec are implemented. Traffic capture, replay, fairness policies, shadow mode, and Router export are planned for subsequent milestones.

InferLab helps AI infrastructure teams answer a risky question before changing production routing:

> What would this candidate scheduling policy have done to latency, fairness, cache affinity, and cost on the exact same workload?

It captures privacy-safe workload metadata, replays an identical trace against multiple policies, checks explicit SLO and fairness invariants, and compares a candidate policy with real production decisions in non-interfering shadow mode.

## Why InferLab exists

Production systems such as the [llm-d Router](https://github.com/llm-d/llm-d-router), [Kubernetes Gateway API Inference Extension](https://github.com/kubernetes-sigs/gateway-api-inference-extension), [vLLM](https://github.com/vllm-project/vllm), and [SGLang](https://github.com/sgl-project/sglang) already serve and route inference traffic. InferLab is not another model server or generic gateway. It is the development and evidence layer around those systems.

| Existing serving layer | InferLab validation layer |
| --- | --- |
| Routes live requests | Replays captured scheduling metadata deterministically |
| Optimizes current traffic | Compares counterfactual policies on identical traffic |
| Exposes operational metrics | Proves SLO, fairness, starvation, and capacity invariants |
| Runs the selected policy | Shadow-tests candidates without changing request routing |
| Configures filters and scorers | Exports validated policies to production integration targets |

The first specialization is **multi-tenant admission control, bounded fairness, and SLO-aware scheduling**—areas where average latency alone can hide serious regressions.

## Target workflow

```text
OpenAI-compatible traffic
          │
          ▼
privacy-safe metadata trace ──────┐
                                  ▼
                         deterministic replay
                     ┌────────────┼────────────┐
                     ▼            ▼            ▼
                round-robin   least-work    fair-SLO
                     └────────────┼────────────┘
                                  ▼
                     metrics + invariant checks
                                  │
                                  ▼
                      shadow validation report
                                  │
                                  ▼
                        llm-d Router integration
```

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

See the [architecture](docs/architecture.md) for determinism, state, privacy, and integration boundaries.

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
./bin/inferlab policies
```

The current CLI reports the built-in baseline policy. Replay commands will land after the deterministic event engine, so the public interface is not built on placeholder behavior.

## Engineering standards

InferLab treats reproducibility and correctness as product features:

- deterministic scheduling inputs and stable tie-breaking;
- mandatory explanations for every endpoint selection;
- privacy-safe capture with no raw prompts by default;
- unit, race, fuzz, golden, integration, and failure-injection tests;
- explicit fairness and SLO assertions, not dashboard-only evaluation;
- calibrated simulation with prediction error published beside results;
- versioned schemas and compatibility policy;
- least-privilege CI, vulnerability scanning, and supply-chain provenance;
- honest benchmarks with committed workloads, configuration, and methodology.

See [CONTRIBUTING.md](CONTRIBUTING.md) for the local merge gate and review expectations.

## Project documentation

- [Architecture and component boundaries](docs/architecture.md)
- [Trace format and privacy contract](docs/trace-format.md)
- [Design principles](docs/design-principles.md)
- [Security policy](SECURITY.md)
- [Governance](GOVERNANCE.md)
- [Support](SUPPORT.md)

## Contributing

Issues and pull requests are welcome. For substantial changes, open a design issue before implementation so contracts, benchmarks, and integration assumptions can be reviewed together. All participants must follow the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

Apache License 2.0. See [LICENSE](LICENSE).
