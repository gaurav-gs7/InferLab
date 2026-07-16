# Evidence adapters and normalization

InferLab adapter protocol v1 turns one exact producer report contract into a lossless, source-neutral evidence envelope. It does not infer semantics from field names. A mapping is valid only when the producer tool, tool version, report schema, metric name, metric definition, and source unit all match a declared capability.

## Trust boundary

Producer reports and external adapter processes are untrusted. The implementation therefore:

- accepts one JSON value, bounded to 8 MiB and 64 nesting levels;
- rejects duplicate fields, unknown fields, trailing values, invalid numbers, mutable producer aliases, and undeclared metrics;
- rejects complete inputs without ordered RFC 3339 start/finish timestamps or a complete runtime signature;
- caps capability declarations at 4,096 metrics and rejects control characters in protocol failure messages;
- launches external adapters directly without a shell;
- supplies an empty environment unless the caller explicitly opts into inheritance;
- propagates context cancellation, caps stdout at 8 MiB by default, caps diagnostic stderr, and applies a bounded process wait;
- requires an immutable adapter revision and exact producer identity in capability discovery; and
- validates the complete normalized response again inside the core process.

The protocol schemas are published under [`schemas/adapter/v1`](../schemas/adapter/v1). A request contains `schema`, `schema_version`, `request_id`, an operation, and—only for `normalize`—an adapter input. The process reads one request from stdin and writes one response to stdout. Diagnostics belong on stderr. A response contains exactly one of capabilities, a normalized report, or a structured failure.

## Lossless normalization

A normalized report contains three connected views:

1. `originals` retains every accepted producer name, definition, numeric value, unit, and sample count.
2. `mappings` records the exact scale, offset, target name, target semantic identifier, value, and unit.
3. `envelope` is the source-neutral evidence used by downstream validity and policy code.

The raw report's exact byte digest is both `input_digest` and the envelope's `raw-report` artifact digest. Validation fails if any original, mapping, normalized metric, sample count, adapter identity, or artifact binding disagrees. Canonical JSON and a stable SHA-256 digest make the result reproducible.

Current fixture conversions include:

| Source | Normalized form |
| --- | --- |
| seconds or microseconds | milliseconds |
| percent | ratio or probability in `[0,1]` |
| USD per 1,000 tokens | USD per 1,000,000 tokens |
| tokens, requests, tokens/second | tokens, count, tokens/second |

Semantic identity is independent of units. GuideLLM TPOT (`output-phase-duration-per-generated-token-v1`) and inference-perf ITL (`adjacent-output-token-gap-v1`) remain different series even after both are expressed in milliseconds. Downstream code must join on metric name *and* semantics, never a display label alone.

## Classification integrity

The GuideLLM and inference-perf fixture adapters accept only `observed` inputs with an observed runtime signature. The generic `NewPredictedAdapter` constructor creates predicted-only adapters; the built-in analytical fixture uses a declared runtime signature. Neither an adapter nor the out-of-process response can turn predicted evidence into observed evidence without failing input, envelope, report, or conformance validation.

## Built-in conformance projections

The repository currently ships three small adapters:

- `guidellm-fixture-v1` pins `guidellm@0.6.0` and the repository's `guidellm-benchmark-v1` conformance projection;
- `inference-perf-fixture-v1` pins `inference-perf@0.1.0` and the repository's `inference-perf-results-v1` conformance projection; and
- `predicted-json-v1` pins a tiny analytical fixture and demonstrates the generic predicted-only adapter constructor.

The committed files under [`pkg/adapter/testdata`](../pkg/adapter/testdata) are intentionally narrow public-safe conformance projections, not claims that InferLab can parse every native upstream report revision. Full native producer support requires captured upstream fixtures, maintainer review of metric definitions, and a new immutable adapter revision.

## CLI

```bash
go run ./cmd/inferlab adapter list
go run ./cmd/inferlab adapter capabilities guidellm-fixture-v1
go run ./cmd/inferlab adapter normalize guidellm-fixture-v1 examples/guidellm-adapter-input.json > normalized.json
go run ./cmd/inferlab adapter validate normalized.json
go run ./cmd/inferlab adapter digest normalized.json
```

The normalize command writes canonical JSON to stdout and never mutates the input. Redirect to a temporary file and atomically rename it when persistence is required.

## Adding an adapter

An adapter contribution must provide an immutable identity, exact producer identity, explicit classification set, one-to-one mapping table, public-safe producer fixtures, deterministic conformance tests, malicious-input tests, and documentation of every metric definition. An adapter must reject unknown fields or metrics; silent dropping, heuristic field matching, and relabelling predictions as observations are release blockers.
