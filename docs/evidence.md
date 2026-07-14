# Runtime identity and evidence contract

Day 4 establishes the boundary between evidence production and release evaluation. The contracts identify what actually ran, preserve where every value came from, and prevent stale or incomplete evidence from silently matching a new configuration.

## Runtime signature

`inferlab.runtime-signature` version `1.0` records 17 material dimensions:

- model identifier and immutable revision;
- tokenizer identifier and immutable revision;
- quantization name and configuration digest;
- engine name, immutable revision, and container image digest;
- CUDA and driver versions;
- GPU SKU, count, and topology;
- scheduler name and configuration digest; and
- the canonical set of material kernel names, versions, and configuration digests.

The `origin` field is either `declared` or `observed`. Declared identity states intent. Observed identity records what an evidence producer actually inspected. Copying a declared value into an observed signature without observing it would be false provenance.

Material fields may be absent only to represent unknown identity. Unknown is not a wildcard. A signature containing unknown dimensions can be stored, hashed, and attached to partial evidence, but it cannot exactly match a complete signature.

Container tags are rejected; an OCI-style image must contain a SHA-256 digest. Model, tokenizer, engine, adapter, and transformation revisions use immutable 40- or 64-character lowercase hexadecimal identities. Configuration blobs are referenced by `sha256:` digest rather than embedded as ambiguous strings.

## Compatibility

`CompareRuntimeSignatures` returns:

- `exact`: every material dimension is known and equal;
- `compatible-by-policy`: every difference is known and explicitly ignored by a named, versioned policy;
- `incompatible`: at least one known difference is not permitted; or
- `unknown`: no unignored mismatch exists, but at least one side lacks material identity.

An unignored known mismatch takes precedence over unknown identity because it is already sufficient to reject reuse. Unknown identity takes precedence over compatibility exceptions. Policies must list known dimensions in canonical sorted order; duplicate or invented dimensions are rejected.

Origin is intentionally not a compatibility dimension: declared baseline identity is expected to be compared with observed run identity. It remains part of the canonical signature digest, so the two documents cannot be substituted for one another.

## Evidence envelope

`inferlab.evidence` version `1.0` binds evidence to:

- classification: `observed`, `predicted`, `derived`, or `asserted`;
- completeness: `complete` or `partial`;
- producer tool, exact version, report schema, adapter, and immutable adapter revision;
- runtime and workload identities;
- attempt number and RFC 3339 timestamps;
- explicitly named metric semantics, units, values, and sample counts;
- raw artifact names, media types, and SHA-256 digests; and
- parent evidence and transformation identity for derived values.

Complete evidence requires a finished timestamp, at least one metric, at least one artifact, and no unknown runtime dimensions. Observed evidence requires an observed runtime signature. Derived evidence requires parent digests and an immutable transformation identity. Partial evidence remains representable for diagnostics but fails closed for claims requiring complete evidence.

The v1 unit vocabulary is deliberately small: `count`, `milliseconds`, `probability`, `ratio`, `seconds`, `tokens`, `tokens_per_second`, and `usd`. Adapters must convert deliberately and retain their source artifacts; strings such as `ms` or metrics without versioned semantics are rejected.

## Canonical identity

Canonicalization sorts semantically unordered kernels, metrics, artifacts, and parent digests before JSON encoding and SHA-256 hashing. It does not sort sequences whose order carries meaning. The resulting digest identifies the exact normalized document; it does not authenticate it or prove the underlying measurement.

## Transitive invalidation

`ValidityGraph` is an append-only dependency DAG. Dependencies must exist before a node is added, which prevents cycles. When a runtime, workload, artifact, or evidence node becomes invalid, breadth-first propagation returns every affected node once with its deterministic shortest dependency path and reason.

Example explanation:

```text
runtime -> observation -> claim: container digest changed
```

Signing and persistent safety-case graphs arrive later. Day 4 provides deterministic invalidation semantics without claiming that an in-memory graph is already a complete safety case.

## Limits and trust boundary

- Evidence JSON is limited to 4 MiB and 64 nesting levels and decoded with duplicate-field, unknown-field, and trailing-value rejection.
- Metrics, artifacts, parents, kernels, identifiers, and GPU counts have explicit bounds.
- Non-finite metric values, duplicate semantics, duplicate artifacts, invalid probabilities, ambiguous units, and backwards timestamps fail validation.
- Evidence artifacts are content references; the envelope does not fetch them or perform external actions.
- A budget or evidence document never authorizes cloud, quota, support, billing, deployment, or repository changes.

The normative schemas are [`runtime-signature.schema.json`](../schemas/evidence/v1/runtime-signature.schema.json) and [`envelope.schema.json`](../schemas/evidence/v1/envelope.schema.json). The examples use synthetic identities and digests to demonstrate the contracts; they are not benchmark results or launch manifests.
