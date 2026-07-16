# Signed safety cases

InferLab safety cases are deterministic, content-addressed bundles for reviewing one release-gate decision offline. They preserve the gate's exact claims and gaps; signing does not turn a prediction into an observation or make incomplete evidence sufficient.

## Closure model

The v1 manifest binds:

- the canonical inference-change, evaluation, and gate-result digests;
- the exact `PASS`, `BLOCK`, or `INCONCLUSIVE` decision;
- an exact projection of every gate finding and evidence reference;
- generated evidence gaps and operator-declared limitations; and
- byte size, SHA-256 digest, media type, role, and relative path for every artifact.

Assembly replays the deterministic gate and rejects a supplied result that differs. Every raw artifact digest named by an evidence envelope must resolve to a `role: evidence` file in the closure. A `BLOCK` case must include at least one valid counterexample document. Verification rehashes all files, replays the gate again, reconstructs claims and gaps, and verifies a detached Ed25519 signature over domain-separated canonical manifest bytes.

The verifier is deliberately offline. It does not contact benchmark producers, GitHub, AWS, a cluster, or a key service.

## Filesystem boundary

The descriptor and manifest accept only canonical relative paths beneath one case root. Absolute paths, `..` traversal, backslash aliases, symlinks, non-regular files, duplicate names/paths, files above 16 MiB, and cases above 64 MiB fail closed. Input JSON is bounded, single-value, duplicate-key rejecting, depth bounded, and unknown-field rejecting.

Artifact verification protects review integrity on a stable local bundle. It is not a defence against an attacker who can concurrently replace the verifier process, kernel, or case directory; use an immutable medium or trusted build environment for adversarial custody.

## CLI

Build the CLI, produce a gate result, assemble the closure, create or supply an Ed25519 key, sign, and verify:

```bash
make build

./bin/inferlab evaluate case/gate-evaluation.json case/gate-result.json
./bin/inferlab safety-case assemble case/descriptor.json case/safety-case.json
./bin/inferlab safety-case keygen case/private.pem case/public.pem
./bin/inferlab safety-case sign case/safety-case.json case/private.pem case/safety-case.sig.json
./bin/inferlab safety-case verify case/safety-case.json case/safety-case.sig.json case/public.pem case
```

`keygen` refuses to overwrite either key and writes the private key with mode `0600`. Treat local key generation as a development convenience. Production release identities should use an approved protected signing workflow and publish a separately authenticated public-key trust policy. InferLab authenticates the key used; it does not decide who is authorized to approve a release.

The top-level `evaluate` command writes the canonical result before returning:

| Exit | Meaning |
| ---: | --- |
| `0` | `PASS` |
| `3` | `BLOCK` |
| `4` | `INCONCLUSIVE` |
| `1` | invalid input or execution failure; no decision |
| `2` | CLI usage error; no decision |

Do not use `go run` when consuming these process exit codes: the Go tool wraps a child non-zero status. Build and invoke `./bin/inferlab` directly.

## Public proof

Run `make demo-safety-case` to reproduce two intentionally synthetic contract proofs:

- `examples/block-gate.json` uses a raw-byte-bound synthetic observation and an in-region two-request counterexample to produce `BLOCK`/exit `3`.
- `examples/missing-evidence-gate.json` has no observation and produces `INCONCLUSIVE`/exit `4`.

The script generates an ephemeral key, signs both manifests, verifies their complete closures, and deletes the private key. Neither fixture is a production safety claim. No public `PASS` fixture is published because this repository does not yet contain adequate observed target-system evidence.

## Schemas

The assembly descriptor, canonical manifest, and detached signature schemas are published in [`schemas/safety-case/v1`](../schemas/safety-case/v1). Schema validation is useful for interoperability; the Go validator remains authoritative for semantic linkage, path safety, canonicalization, gate replay, and cryptographic verification.
