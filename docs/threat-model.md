# Threat model

## Assets and security goals

InferLab protects release-decision integrity, evidence provenance, workload privacy, bounded local execution, and honest uncertainty. A reviewer must be able to tell which exact intent, runtime, workload, raw bytes, metrics, uncertainty assumptions, policies, and counterexample support each claim.

The core safety properties are fail-closed:

- untrusted documents cannot consume unbounded memory or smuggle duplicate/unknown fields;
- predicted, derived, asserted, partial, stale, future, incompatible, or unknown-runtime evidence cannot satisfy observed-evidence policy;
- similar metric names with different semantics cannot be pooled;
- changing a workload coordinate, uncertainty declaration, gate result, raw artifact, claim, limitation, or signature invalidates the relevant digest or closure; and
- invalid execution is not reported as `BLOCK` or `INCONCLUSIVE`.

## Trust boundaries

| Boundary | Trusted for | Not trusted for |
| --- | --- | --- |
| Inference-change author | Declaring intended scope and policy | Proving what executed |
| Adapter | Parsing one pinned producer format | Upgrading evidence class or producer truthfulness |
| Evidence producer | Its signed/hashed raw output | Release approval |
| Gate core | Deterministic admission and policy evaluation | Benchmark fidelity outside declared evidence |
| Safety-case signer | Authenticating a manifest under one key | Proving key-owner authority or operational safety |
| CI runner | Reproducing public synthetic contracts | Holding production signing secrets |

All current core evaluation and public proof paths are local. They do not mutate cloud accounts, quotas, support cases, clusters, deployments, billing resources, or repository identity.

## Adversaries and controls

### Malicious or corrupt inputs

Inputs may contain oversized values, deep nesting, duplicate keys, trailing values, non-finite numbers, ambiguous units, mutable identifiers, invalid timestamps, or graph cycles. Strict bounded decoders, immutable-identity validators, collection ceilings, canonical digests, and adversarial/fuzz tests reject them.

### Provenance and artifact substitution

An attacker may replace a raw benchmark report, evaluation, gate result, counterexample, or summary. Evidence envelopes bind raw byte digests; manifests bind every artifact digest and byte size; assembly and verification replay the gate; exact claims and gaps are reconstructed; detached Ed25519 signatures bind canonical manifest bytes.

### Filesystem attacks

An artifact descriptor may attempt path traversal, absolute paths, symlink escape, non-regular files, aliasing, or oversized closure. Canonical relative paths, an OS-rooted filesystem handle, containment checks, `Lstat`/opened-file identity checks, regular-file enforcement, per-file/total bounds, and duplicate rejection mitigate these attacks. A hostile process with concurrent write access to the verifier's directory or kernel remains outside the boundary; verify immutable copied bundles in a trusted environment.

### Adapter process attacks

An external adapter may hang, emit excessive output, inherit secrets, invoke a shell, or lie about capabilities. The runner uses direct execution, no shell, an empty environment by default, cancellation, bounded stdout/stderr, protocol validation, and pinned identities. Sandbox strength still depends on the host; use OS/container isolation for untrusted third-party binaries.

### Privacy leakage

Raw prompts, responses, credentials, and tenant names must not enter public fixtures or default traces. Trace identities use domain-separated keyed fingerprints and allowlisted metadata. Artifact publication remains an operator responsibility: hashing sensitive content does not make it safe to publish.

### Signing and key confusion

Signatures use Ed25519, PKCS#8 private keys, PKIX public keys, a domain separator, a SHA-256 public-key identifier, and a detached manifest digest. The project does not yet implement organizational approver policy, threshold signing, revocation, transparency logs, timestamp authorities, or hardware-backed keys. Those are release-integration responsibilities, not implied by cryptographic validity.

### CI and supply chain

GitHub workflows have explicit `contents: read` permissions, do not consume repository secrets for public proofs, verify module contents, run tests/race/vet/vulnerability/CodeQL checks, and upload only ephemeral public proof artifacts. Dependabot covers Go modules and Actions. Tag protection, action SHA pinning, release provenance/attestations, and protected production signing are release-readiness items.

## Residual risk and non-claims

InferLab cannot prove that a benchmark reflects future production traffic, that an observed runtime was reported honestly by a compromised producer, that all relevant policies were authored, or that a cryptographic key belongs to an authorized approver. A `PASS` means the declared mandatory policies are supported inside the declared evidence envelope. It is not a guarantee of incident-free production behavior.
