# Release-readiness audit

Audit date: 2026-07-16. Current disposition: **pre-alpha; do not tag v0.1 yet**.

## Completed engineering proof

- Strict versioned change, runtime, evidence, adapter, gate, counterexample, and safety-case contracts.
- Canonical SHA-256 identities, exact runtime compatibility, raw-artifact closure, deterministic gate replay, and detached Ed25519 verification.
- Stable decision exit codes and public-safe reproducible `BLOCK` and `INCONCLUSIVE` cases.
- Unit, race, fuzz, golden/conformance, integration, path-tamper, signature-tamper, and missing-provenance coverage.
- Least-privilege CI, CodeQL, vulnerability scanning, module verification, dependency update policy, and public decision artifacts.
- Apache-2.0 license, contribution/security/governance/support documents, issue templates, and an honest pre-alpha scope.

## Release blockers

1. **Owner-approved name:** `InferLab` conflicts with Doubleword's Inference Lab. The repository/module/import path, schemas, CLI presentation, badges, and links must migrate only after the maintainer selects the replacement.
2. **Observed target-system proof:** public fixtures are synthetic. A v0.1 claim needs sanitized, compatible, raw observed evidence from the declared target runtime and a documented benchmark methodology.
3. **No public PASS yet:** a PASS example must remain absent until all mandatory policy regions have adequate observed evidence. Adding a fabricated PASS would be a release blocker, not polish.
4. **Signing trust policy:** define authorized release keys, protection, rotation/revocation, and branch/tag protection. Development key generation is not an organizational approval system.
5. **Supply-chain release policy:** pin third-party Actions to reviewed immutable commit SHAs and add release provenance/attestation only when the final repository identity and tag policy exist.
6. **Clean-room acceptance:** reproduce `make check`, a bounded fuzz run, `make demo-safety-case`, vulnerability scanning, CodeQL, and both GitHub checks from a fresh clone after the rename migration.

## Naming migration checklist

After the owner chooses a non-conflicting name:

- rename the GitHub repository and Go module/import paths in one reviewed migration;
- update schema identifiers, CLI/help text, package comments, documentation, workflow badges, issue links, and security-reporting references;
- search case-insensitively for both `InferLab` and `inference lab` and review every remaining use;
- verify redirects are not the only mechanism relied on by published schemas;
- rerun clean-room acceptance and security scans; and
- tag v0.1 only when every blocker above is closed.

No repository rename, release tag, cloud action, deployment, quota request, support case, billing mutation, or production approval is authorized by this audit.
