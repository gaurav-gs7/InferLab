# ADR 0001: own release assurance, not simulation

- Status: accepted
- Date: 2026-07-14

## Context

The original design placed a custom discrete-event simulator, scheduler suite, and calibrated worker model on the critical path. That direction overlaps with maintained systems including Doubleword Inference Lab, Microsoft Vidur, KAIST LLMServingSim, Alibaba InferSim, GuideLLM, and Kubernetes SIGs inference-perf. Reimplementing their strongest capabilities would consume the project while producing a weaker simulator or benchmark tool.

The less-served problem is deciding whether heterogeneous evidence is valid and sufficient for one exact inference-infrastructure change. Benchmark and simulator output becomes stale when material runtime identity changes. Similar metric names can have incompatible semantics. Percentile regressions rarely produce a minimal reproducer. Conventional CI also lacks an explicit outcome for insufficient evidence.

## Decision

InferLab will be a release-assurance plane with replaceable evidence producers.

The core will own:

- immutable change intent and exact observed runtime signatures;
- versioned, loss-aware evidence adapters and normalization;
- transitive evidence invalidation, drift, coverage, and uncertainty;
- inference-specific fault evidence contracts;
- bounded counterexample minimization and verification;
- deterministic `PASS`, `BLOCK`, and `INCONCLUSIVE` policies; and
- signed evidence graphs and review artifacts.

GuideLLM and inference-perf are the first intended observed-evidence adapters. Simulators remain external predicted-evidence producers. A tiny deterministic fixture backend may support tests but will not be presented as a vLLM performance simulator.

The existing scheduler SDK and trace codec remain supported foundations where they serve evidence fixtures, workload identity, or future routing-policy claims. Scheduler and simulator breadth is not pursued independently.

## Consequences

Positive consequences:

- effort targets a more defensible integration and decision problem;
- mature benchmark and simulation projects can improve without forcing a rewrite of the core;
- evidence provenance and validity remain consistent across producers;
- the first useful release can demonstrate BLOCK and INCONCLUSIVE without claiming universal performance prediction.

Costs and risks:

- upstream report formats and metric definitions require ongoing compatibility work;
- adapters must isolate heavy or conflicting third-party dependencies;
- cross-tool normalization can introduce semantic loss and therefore must fail closed;
- the product depends on credible upstream evidence producers for end-to-end demonstrations; and
- the working name conflicts with an existing project and must change before v0.1.

## Rejected alternatives

### Build a higher-fidelity simulator

Rejected because established research and industry projects already specialize in simulation fidelity, hardware/network modelling, scheduling, and capacity search. InferLab cannot credibly out-scope all of them within v0.1.

### Build a benchmark dashboard

Rejected because GuideLLM and inference-perf already provide mature load generation, metrics, and reports. Visualization alone does not establish evidence validity or release safety.

### Support one evidence producer only

Rejected because source lock-in would make the core a thin wrapper and prevent independent corroboration. The adapter conformance model is required from the first supported producer.

## Revisit criteria

A built-in execution or simulation component may be proposed only when a written verification requirement cannot be met by an existing maintained project, upstream contribution, or bounded adapter. The proposal must define the gap, evidence semantics, maintenance cost, and why it belongs in the trusted core.
