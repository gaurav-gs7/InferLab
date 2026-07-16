# Contributing to InferLab

Thank you for helping build rigorous release-assurance tooling for LLM inference changes. Contributions of adapters, evidence contracts, traces, runtime identity rules, uncertainty methodology, fault evidence, counterexamples, documentation, and design review are welcome.

## Before starting

- Search existing issues and pull requests.
- Open an issue before substantial API, schema, dependency, architecture, or behavior changes.
- Never attach raw production prompts, responses, secrets, tenant identifiers, or proprietary traces.
- Keep pull requests focused enough to review and revert independently.

## Development setup

Install Go 1.26 or newer, then run:

```bash
make check
make build
```

`make check` formats source, runs static analysis, unit tests, and the race detector. Feature-specific changes may also require fuzz, integration, benchmark, or kind-based tests documented beside that feature.

Before a release candidate, run `make audit`. The extended audit checks the diff and module, enforces aggregate coverage, repeats tests in randomized order, runs the race detector and every fuzz target, cross-builds the CLI for Linux/macOS/Windows on amd64/arm64 where applicable, and reproduces the signed safety-case workflow.

Trace, change, evidence, adapter, gate, parser, or privacy changes must also run `make fuzz`. Override the default campaign length with `FUZZTIME=1m make fuzz`.

## Engineering expectations

- Add tests that fail without the change.
- Preserve deterministic normalization and evaluation; record seeds and never read uncontrolled wall time or global randomness in decision logic.
- Keep observed, predicted, derived, and asserted evidence distinct.
- Pin upstream producer/report versions and add conformance fixtures before claiming support.
- Prefer an upstream contribution or adapter over duplicating an existing benchmark or simulator.
- Return actionable errors with stable sentinel/type matching where callers need it.
- Keep scheduling contracts free of Kubernetes and serving-engine dependencies.
- Treat all trace input as untrusted and resource-bound parsing work.
- Add structured reason codes for new decision factors.
- Bind every decision-relevant field into a canonical digest and evidence-graph dependency.
- Prove that stale, partial, predicted, incompatible, under-sampled, and out-of-region evidence cannot produce `PASS` when changing gate behavior.
- Re-verify minimized counterexamples and report budget exhaustion instead of claiming a global minimum.
- Document observed, predicted, and derived benchmark values separately.
- Update user docs and compatibility notes with behavior changes.

Public API and trace-schema changes require a migration note and explicit compatibility discussion.

## Commits and pull requests

Use clear imperative commits, preferably Conventional Commit prefixes such as `feat:`, `fix:`, `docs:`, `test:`, `perf:`, or `chore:`. A pull request should explain:

- the problem and intended behavior;
- alternatives and trade-offs;
- correctness and failure semantics;
- test evidence;
- performance or compatibility impact;
- privacy/security impact;
- documentation changes.

All commits must include a Developer Certificate of Origin sign-off:

```text
Signed-off-by: Your Name <your.email@example.com>
```

Create it with `git commit -s` after confirming that you have the right to contribute the work under Apache-2.0. See [developercertificate.org](https://developercertificate.org/) for the certificate text.

## Review and merge

Maintainers may request smaller commits, additional evidence, or a design issue before review. Required CI must pass. Changes are squash-merged unless preserving a carefully structured commit series materially improves history.

## Reporting security issues

Do not open public issues for vulnerabilities or sensitive-data exposure. Follow [SECURITY.md](SECURITY.md).
