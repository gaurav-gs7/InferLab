# Competitive landscape and positioning

Last reviewed: 2026-07-14.

This document prevents the project from claiming novelty already delivered elsewhere. It is based on the linked projects' public repositories and documentation, not on private implementation knowledge. Capabilities change; review the upstream source before making a comparison in a release or proposal.

## Existing systems

| Project | What its own documentation establishes | Consequence for this project |
| --- | --- | --- |
| [Doubleword Inference Lab](https://github.com/doublewordai/inference-lab) | Rust discrete-event inference simulator with a vLLM queue/scheduler facsimile, performance modelling, chunked prefill, KV-cache management, workload generation, multiple policies, CLI, and WASM | Do not build or market another generic "Inference Lab" simulator. The working name must change before v0.1. |
| [Microsoft Vidur](https://github.com/microsoft/vidur) | Profile-informed, high-fidelity inference simulation for workload/configuration studies, capacity-per-dollar planning, and scheduling research | Consume compatible output through an adapter; do not make calibrated simulation itself the differentiator. |
| [KAIST LLMServingSim](https://github.com/casys-kaist/LLMServingSim) | vLLM-style scheduling plus hardware/network co-simulation for heterogeneous accelerators, disaggregated memory, MoE, and parallel deployments | Treat sophisticated topology/hardware simulation as an upstream evidence source. |
| [vLLM GuideLLM](https://github.com/vllm-project/guidellm) | SLO-aware benchmarking against real endpoints with production-like workloads, detailed latency/token distributions, reports, and regression-oriented output | Build a strict observed-evidence adapter instead of another load generator or dashboard. |
| [Kubernetes SIGs inference-perf](https://github.com/kubernetes-sigs/inference-perf) | Model-server-agnostic production-scale benchmarking with TTFT/TPOT/ITL, throughput, SLO goodput, staged loads, datasets, and trace replay | Preserve its metric semantics and workload identity; use it as a second observed-evidence producer. |
| [Alibaba InferSim](https://github.com/alibaba/InferSim) | Lightweight analytical TTFT, TPOT, and tokens-per-GPU-second prediction from model computation, hardware, kernels, and communication | Import predictions as predictions with signature/calibration metadata; do not relabel them observations. |

Runtime platforms such as [vLLM](https://github.com/vllm-project/vllm), [SGLang](https://github.com/sgl-project/sglang), [llm-d](https://github.com/llm-d/llm-d), and [AIBrix](https://github.com/vllm-project/aibrix) are adjacent deployment targets and evidence subjects. They are not competitors to the release-decision layer and are not replaced by it.

## Defensible product thesis

The project owns the decision boundary between evidence production and deployment:

1. exact runtime signatures and transitive evidence invalidation;
2. loss-aware normalization across benchmark, simulator, and fault sources;
3. workload coverage, drift, uncertainty, and sample-sufficiency analysis;
4. bounded infrastructure-fault semantics;
5. minimal, verified SLO-breaking counterexamples;
6. deterministic `PASS`, `BLOCK`, or `INCONCLUSIVE` policy gates; and
7. signed, provenance-rich evidence graphs for CI and deployment review.

This is a product thesis, not a current novelty claim. Each item must be demonstrated by code, adversarial tests, and reproducible artifacts before the README describes it as shipped.

## Naming risk

`InferLab` and `Inference Lab` are too close to Doubleword's existing project in the same problem domain. The repository and CLI currently retain the working name to avoid an unapproved identity change. A distinct name is a v0.1 release blocker. Selection should include repository/package availability, trademark screening by the owner where appropriate, CLI ergonomics, and a migration plan for Go import paths and schemas.

## Rules for future scope

- Prefer an adapter when a maintained project already produces the required evidence.
- Build a new execution engine only when a documented verification requirement cannot be satisfied upstream or through normalization.
- Never compare tools using unsupported percentage-overlap or accuracy claims.
- Never imply endorsement by an upstream project.
- Pin producer and report versions in conformance fixtures.
- Keep observed, predicted, derived, and asserted evidence distinct through every report and decision.
