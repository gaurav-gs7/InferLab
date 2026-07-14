# Design principles

## Evidence over demos

A feature is complete when its behavior is specified, tested, measured, and reproducible. A screenshot or happy-path demo is useful documentation, not validation.

## Determinism is an API guarantee

Given the same trace, configuration, seed, calibration data, and InferLab version, replay must produce byte-equivalent canonical results. Any intentional source of nondeterminism is recorded in the report.

## Correctness before optimization

Scheduler changes must preserve endpoint eligibility, quota, fairness, starvation, and deadline invariants before their performance improvements are considered. Benchmarks never waive correctness gates.

## Privacy by construction

Raw prompts and responses are not captured by default. Sensitive metadata has explicit classification, retention, and redaction behavior. Capture failure cannot silently expose content or disrupt production traffic.

## Explain every decision

Endpoint choices carry structured reason codes and finite score contributions. Explanations are stable enough for machine comparison and readable enough for incident review.

## Calibrate claims

Simulation estimates are labeled as predictions. Hardware results identify the backend, model, quantization, accelerator, software versions, workload, warm-up, sample size, and uncertainty. Prediction error is a published metric.

## Integrate at the edge

The core has no Kubernetes or serving-engine dependency. Upstream-specific adapters live at explicit boundaries and are compatibility-tested against pinned versions.

## Boring operations are a feature

Bounded memory, cancellation, timeouts, health endpoints, telemetry, safe defaults, graceful shutdown, schema migration, and actionable errors are part of the product—not release polish.

## Honest project status

Documentation distinguishes implemented, experimental, and planned capabilities. Benchmarks and badges must be reproducible from the referenced revision.
