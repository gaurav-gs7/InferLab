# Production-readiness and product-value report

Audit date: 2026-07-16

Audit baseline: `a3b5439b3b895dfe659952b12af61aed78a39985` plus the hardening changes committed with this report

Repository: [gaurav-gs7/InferLab](https://github.com/gaurav-gs7/InferLab)

Disposition: **credible pre-alpha release-assurance core; not yet a production-ready deployed product**

## Executive verdict

InferLab is not another inference simulator. It is a fail-closed release-assurance layer for LLM inference changes. It normalizes evidence from external producers without erasing provenance, decides whether the exact observed evidence is sufficient for declared SLO/cost/fairness policies, and packages the decision into a content-addressed signed safety case.

The current implementation is unusually disciplined for a pre-alpha portfolio project. The core is deterministic, dependency-light, aggressively bounded, strict about evidence class and runtime identity, and backed by race, fuzz, tamper, canonicalization, conformance, and decision-closure tests. The extended audit passed with 78.6% aggregate statement coverage; the safety-sensitive trace, evidence, gate, and safety-case packages measured 90.4%, 90.2%, 81.0%, and 82.8% respectively.

It is still not honest to call the whole product production grade. There is no authorized end-to-end AWS GPU experiment, no broad native upstream adapter support, no observed target-system `PASS` proof, no automated experiment executor/teardown reconciler, and no organizational signing or release-key policy. GitHub `main` is also unprotected, no repository ruleset exists, Actions are not restricted to pinned sources, and Dependabot security updates are disabled. These are release blockers, not documentation footnotes.

## The real-world problem

An inference platform change can improve average throughput while quietly worsening p99 latency, one tenant's fairness, recovery under replica loss, long-context behavior, or cost per successful token. Benchmark tools report measurements and simulators report predictions, but teams still have to answer harder questions:

1. Does this result describe the exact model, tokenizer, engine, container, driver, accelerator, scheduler, and kernels we plan to ship?
2. Is the result observed, predicted, derived, or merely asserted?
3. Is it fresh, sufficiently sampled, inside the relevant workload/fault region, and statistically decisive?
4. Did every mandatory policy pass, or did an aggregate improvement hide a local regression?
5. Can a reviewer reproduce the decision and verify that evidence or summaries were not substituted?

Without a common assurance layer, these decisions become spreadsheets, dashboards, tribal knowledge, and optimistic release comments. That is slow to review and easy to misrepresent. InferLab turns the decision boundary into strict contracts and reproducible evidence.

## What the project does

| Component | Responsibility | Important guarantees | Current boundary |
| --- | --- | --- | --- |
| Inference-change contract | Pins baseline/candidate intent, workload, policies, faults, and budget ceilings | Strict bounded JSON; immutable model/container identities; deterministic digest; unsupported features fail closed | A budget is not execution authority; v0.1 supports a narrow vLLM/L4 envelope |
| Scheduler SDK | Defines a small explainable policy interface and cluster snapshot contract | Request/snapshot/decision validation; temporal consistency; stable round-robin baseline; concurrent safety | No production control plane or vLLM scheduler facsimile |
| Privacy-safe trace | Captures scheduling metadata without prompt/response content | Domain-separated HMAC pseudonyms; bounded streaming; sensitive-field rejection; contiguous sequence and non-decreasing arrivals | Equality patterns and operational metadata can remain sensitive |
| Runtime identity | Identifies the material system that produced evidence | Exact model/tokenizer/engine/container/platform/scheduler/kernel identity; immutable revisions; unknown is never a wildcard | V1 permits only a reviewed driver-version compatibility waiver |
| Evidence envelope | Preserves evidence class, provenance, metrics, artifacts, and transformations | Complete observed evidence requires observed complete runtime and ordered timestamps; canonical content identity | Hashes prove byte identity, not producer honesty |
| Adapters | Normalize pinned producer contracts without semantic guessing | Raw-byte binding; originals and conversions retained; classification integrity; process/output limits; empty environment by default | Built-ins are narrow conformance projections, not full upstream parsers |
| Validity graph | Tracks runtime/workload/artifact/evidence/claim dependencies | Append-only DAG; deterministic transitive invalidation paths | In-memory invalidation is not organizational approval |
| Uncertainty gate | Produces `PASS`, `BLOCK`, or `INCONCLUSIVE` | Only fresh, complete, compatible observed evidence can pass; exact metric semantics; sample and uncertainty requirements; any block wins | V1 uses normal/Wilson approximations and declared regions; it is not a universal statistical proof |
| Counterexample minimizer | Reduces a reproducing failure through a caller-supplied oracle | Deterministic bounded search; budget accounting; final repeated verification; incomplete search is explicit | Produces a local minimum under declared transformations, not a global minimum |
| Safety case | Packages one offline-reviewable decision closure | Gate replay; raw artifact closure; SHA-256 identities; path/symlink/size checks; domain-separated Ed25519 signature | Valid signature proves possession of one key, not that its owner is an authorized approver |
| CLI and CI | Exposes stable automation behavior | Stable decision exit codes, atomic outputs, exclusive key creation, broken-output detection, Linux CI, CodeQL, vulnerability scan, signed public proofs | No release binary provenance/SBOM/tag policy yet |

## How a decision is produced

1. A strict inference-change document declares the exact candidate and the policies that matter.
2. A benchmark, simulator, or separately authorized fault run produces a pinned report.
3. An adapter retains the original values and raw-report digest while making every unit and semantic conversion explicit.
4. The evidence envelope binds that normalized result to classification, runtime, workload, attempt, timestamps, artifacts, and transformations.
5. The gate rejects ineligible evidence, checks coverage and uncertainty for each mandatory rule, and emits one deterministic decision with stable reason codes.
6. A reproducing failure can be minimized and reverified through a bounded oracle.
7. Safety-case assembly replays the gate instead of trusting the supplied result, closes over raw artifacts, projects exact findings/gaps, and signs the canonical manifest.
8. An offline reviewer rehashes all artifacts, replays the gate, reconstructs claims, and verifies the detached signature.

This separation is the product's main design strength: producers generate evidence; InferLab decides whether that evidence is admissible and sufficient. It does not invent benchmark results or silently upgrade predictions into observations.

## Audit scope and method

The audit reviewed all 83 Go files, nine packages plus the CLI, every published JSON Schema, examples, shell scripts, GitHub workflows, and OSS governance/security documentation. The completed audit revision contains 147 tracked files and no third-party Go runtime dependencies.

The phrase “all possible edge cases” cannot be proven for a non-trivial program. This audit used layered evidence instead: code review of every trust boundary, table-driven adversarial tests, deterministic golden tests, race detection, randomized repeated execution, coverage, all eight fuzz targets, cross-compilation, CLI black-box workflows, filesystem/signature tamper tests, and read-only AWS/GitHub control-plane inspection. Remaining untested state is listed explicitly below.

### Executed verification

| Check | Exact execution | Result |
| --- | --- | --- |
| Module integrity | `go mod verify` | Pass; all modules verified |
| Formatting/diff hygiene | `gofmt -l .`, `git diff --check` | Pass |
| Standard static analysis | `go vet ./...` | Pass |
| Extended static analysis | `golangci-lint 2.11.3 run ./...` | Pass; zero issues after CLI output hardening |
| Unit/integration/conformance | `go test -coverprofile=... -covermode=atomic ./...` | Pass |
| Race detection | `go test -race ./...` | Pass |
| Order dependence | `go test -shuffle=on -count=3 ./...` | Pass |
| Fuzzing | Eight fuzz targets, 2 seconds each in the recorded audit | Pass; no crash or invariant failure |
| Cross-build | Linux amd64/arm64, macOS amd64/arm64, Windows amd64; `CGO_ENABLED=0` | Pass; Linux outputs are statically linked ELF binaries |
| Public decision proof | `bash scripts/demo-safety-case.sh dist/release-audit/safety-gate` | Pass; reproduced, signed, and verified `BLOCK` and `INCONCLUSIVE` closures |
| Representative CLI | Change/runtime/evidence validation and adapter normalization/validation | Pass |
| Clean-directory acceptance | Copied source without Git/build/cache artifacts; ran `make check`, `make build`, and `make demo-safety-case` with fresh caches | Pass |
| JSON Schemas | Published-schema tests plus `jq empty` on changed schemas | Pass |
| Basic committed-secret pattern scan | Private-key header and AWS access-key patterns over tracked files | No match; not a replacement for platform secret scanning |
| Remote security workflows at baseline | CI, CodeQL, safety-gate proof, govulncheck job | Last baseline runs passed; current hardening revision requires post-push confirmation |

The reproducible command is:

```bash
FUZZTIME=2s make audit
```

Release candidates should use a materially longer fuzz duration, for example `FUZZTIME=10m make audit`, in a fresh Linux environment.

### Statement coverage

| Package | Coverage |
| --- | ---: |
| `cmd/inferlab` | 47.6% |
| `internal/strictjson` | 74.5% |
| `pkg/adapter` | 72.7% |
| `pkg/change` | 83.2% |
| `pkg/evidence` | 90.2% |
| `pkg/gate` | 81.0% |
| `pkg/policy/roundrobin` | 76.2% |
| `pkg/safetycase` | 82.8% |
| `pkg/scheduler` | 95.9% |
| `pkg/trace` | 90.4% |
| **Aggregate** | **78.6%** |

Coverage is useful evidence, not proof of correctness. The CLI percentage is the clearest test-depth opportunity; most happy/error commands are exercised, but every filesystem failure permutation is not. The audit gate enforces a 75% aggregate floor so future work cannot silently erase the current baseline.

## Edge-case and failure-mode coverage

| Area | Cases exercised | Result |
| --- | --- | --- |
| JSON ingestion | Nil readers, empty input, oversized input, excessive nesting, duplicate keys, unknown keys, trailing values, malformed JSON, invalid UTF-8 | Rejected without panic |
| Numeric/resource safety | NaN/infinity, negative values, invalid probabilities, zero samples, token overflow/underflow boundaries, per-record/stream/count limits | Rejected or bounded |
| Trace privacy | Raw/nested prompt/content fields, unsafe metadata keys/values, default-deny allowlist, HMAC key bounds, model/domain/key separation | Sensitive fields rejected; pseudonyms deterministic within a key domain |
| Trace ordering/versioning | Sequence gaps/duplicates, decreasing arrival offsets, unsupported major, unknown current-version field, future-minor extension | Current contract strict; future minor inspectable but not re-emittable |
| Change intent | Mutable images/revisions, unsupported engine/accelerator/quantization, credential-bearing trace URI, duplicate tenants/faults, unordered fault grids, unsafe violation risk | Rejected with typed errors |
| Scheduler | Missing/invalid request fields, inconsistent capacity, duplicate models/endpoints, future observations, unhealthy/draining/wrong-model targets, cancellation, 1,000 concurrent calls | Contract held; race test passed |
| Runtime/evidence | Every material runtime mutation, unknown identity, mutable version patterns, backwards/missing timestamps, partial/derived classification, canonical order | Exact mismatch invalidates; incomplete evidence remains representable but inadmissible for pass |
| Compatibility | Unknown, mismatch, exact, explicit driver policy, invented/duplicate/unsorted/non-waivable dimensions | Fails closed; model/container/scheduler/kernel waivers rejected |
| Adapter boundary | Duplicate/unknown/trailing producer input, wrong producer, missing/duplicate/semantically changed metrics, ambiguous units, predicted-to-observed relabel, incomplete timestamps/runtime, oversized capability declaration | Rejected; raw bytes and conversion mapping remain bound |
| External adapter process | Cancellation, excessive stdout, bounded stderr, mismatched response identity, shell-free invocation, empty-default environment | Runner terminates/fails closed |
| Gate admission | Predicted, partial, stale, future, incompatible, unknown-runtime, out-of-region, wrong workload, missing metric/uncertainty, under-sampled evidence | Cannot produce `PASS` |
| Gate decision | All supported policy families, confidence overlap, violation risk, many attempts, mandatory regression hidden by aggregate gains, order determinism | Any violation blocks; any unresolved rule is inconclusive; all-pass unit fixture passes |
| Counterexample | Determinism, subset/numeric reductions, long-context reduction, evaluation budget, cancellation, non-failing input, failed final reproduction | No unverified minimum returned |
| Safety-case closure | Result/claim/artifact tamper, missing raw evidence, wrong/tampered key/signature, unsafe traversal, symlink/file alias, missing counterexample, counterexample outside blocked region | Assembly/verification fail closed |
| CLI | Usage errors, missing files, all main validation/digest paths, stable decision exit codes, atomic result output, exclusive key creation, failed stdout/stderr | Stable documented behavior |

## Defects found and remediated during this audit

| Severity | Finding | Remediation |
| --- | --- | --- |
| High | Change documents accepted duplicate JSON keys with last-value-wins behavior | Routed ingestion through the shared duplicate/depth/single-value validator and added regression coverage |
| High | A valid change allowed violation probability up to `1.0` while the gate allowed only `0.5` | Unified code/schema contract at `(0, 0.5]` |
| High | Compatibility policy could generically waive model, container, scheduler, kernel, or accelerator differences | Restricted V1 policy waivers to driver version only; schema/docs/tests aligned |
| Medium | Runtime/producer identities rejected only a short alias list | Added wildcard/range/nightly/snapshot/canary/edge/unstable rejection |
| Medium | Adapter input validation deferred timestamps and complete-runtime checks until later normalization | Enforced ordered RFC 3339 timestamps and complete identity at the adapter boundary |
| Medium | Capability metric arrays had no semantic count ceiling; failure strings allowed log-injection controls | Added a 4,096 metric ceiling and UTF-8/control-safe failure validation |
| Medium | Trace decoder tolerated unknown fields on the current schema and did not enforce stream ordering | Current version is strict; sequence and arrival invariants now enforced by encoder and decoder |
| Medium | Snapshot endpoint observations could be later than the snapshot capture time | Added temporal consistency validation |
| Medium | Safety-case reads relied on path resolution and same-file checks but did not open through an OS-rooted handle | Added `os.OpenRoot` containment before opening artifacts |
| Low | CLI ignored output errors, so a broken pipe could preserve a success/decision exit code | Added output tracking and failure exit semantics; extended lint now passes |
| Low | Production verification steps were spread across commands | Added `make audit`, a reusable verification script, and an opt-in AWS CodeBuild buildspec |

## AWS validation status

AWS was inspected conservatively. No resource, IAM policy, quota, support case, deployment, or charge was created or changed.

### Demonstrated

- AWS CLI credentials authenticated through a read-only STS call; account identifiers were suppressed from audit output.
- A region is configured locally.
- A read-only EC2 catalog query reported `g6.xlarge` offerings in five `us-east-1` Availability Zones on 2026-07-16.
- The CLI cross-builds as a statically linked Linux amd64 and arm64 binary with no third-party runtime dependencies.
- [`buildspec.aws.yml`](../buildspec.aws.yml) provides a reproducible, credential-free CodeBuild validation recipe using the AWS-documented buildspec/runtime mechanism.

AWS publishes `g6.xlarge` with one NVIDIA L4 GPU and 24 GB GPU memory, 4 vCPUs, and 16 GiB system memory in its [G6 specification](https://aws.amazon.com/ec2/instance-types/g6/). AWS also documents buildspec version `0.2` and runtime selection in the [CodeBuild buildspec reference](https://docs.aws.amazon.com/codebuild/latest/userguide/build-spec-ref.html) and [runtime-version reference](https://docs.aws.amazon.com/codebuild/latest/userguide/runtime-versions.html).

### Not demonstrated

- The current revision was not executed in AWS CodeBuild.
- No EC2 GPU was launched; quota and point-in-time capacity were not tested.
- No vLLM container/model was started and no NVIDIA driver/CUDA/kernel identity was observed.
- No native benchmark report, fault campaign, teardown record, or cost reconciliation was produced.
- No AWS-derived observed `PASS` safety case exists.

Therefore “working on AWS” is currently supported at the build/target/read-only-account-readiness level, not at the end-to-end inference-experiment level. Closing that gap requires explicit authorization for a concrete ephemeral deployment plan, spend ceiling, IAM role, data handling, and teardown verification.

## GitHub and OSS readiness

### Strong controls already present

- Public Apache-2.0 repository with contribution, DCO, security, governance, support, conduct, issue, and PR guidance.
- Read-only workflow permissions for CI/public proofs; CodeQL has only the required security-event write.
- Module verification, formatting, vet, unit/coverage, race, vulnerability, CodeQL, and signed-decision workflows.
- Secret scanning and push protection enabled.
- Zero open CodeQL alerts were reported during the read-only inspection.
- Public examples are synthetic and clearly avoid a fabricated production `PASS` claim.

### Repository controls still below production OSS expectations

- `main` has no branch protection and the repository has zero rulesets.
- GitHub Actions allows all actions and does not require SHA pinning.
- Dependabot security updates are disabled.
- Third-party Actions use mutable major tags rather than reviewed commit SHAs.
- The public repository has no description or topics, reducing discoverability.
- There is no tagged supported release, changelog/release process, signed release artifact, SBOM, or provenance attestation.
- The working name conflicts with an existing inference project and should be replaced before v0.1.

Changing these hosted settings is an external mutation and was intentionally not done by this audit.

## Production blockers and prioritized next work

### P0 — required before a production or v0.1 claim

1. Choose and execute the owner-approved non-conflicting name/module/schema migration.
2. Protect `main` with required CI, CodeQL, and safety-gate checks; add a ruleset, review requirements, and restricted force-push/deletion policy.
3. Pin allowed Actions to reviewed commit SHAs and enable Dependabot security updates.
4. Implement at least one real native observed-producer integration, rather than only conformance projections.
5. Run a separately authorized, budgeted, teardown-verified AWS L4 experiment and publish sanitized observed evidence/methodology.
6. Produce the first honest `PASS` case only after every mandatory region/rule is covered by adequate observed evidence.
7. Define release-key authorization, storage, rotation, revocation, and reviewer policy.

### P1 — required for serious organizational adoption

1. Add an experiment-runner boundary with explicit authorization tokens, hard cost/time ceilings, idempotent teardown, and usage reconciliation.
2. Add performance/resource benchmarks for worst-case bounded JSON, trace streaming, graph evaluation, and 512-evidence gate documents; unit limits alone do not characterize latency or memory.
3. Run long-duration fuzzing and clean-room Linux acceptance on release candidates.
4. Add signed release binaries, checksums, SBOM, SLSA-style provenance, changelog, and compatibility/migration policy.
5. Extend signing to organizational trust policy or integrate with an approved KMS/HSM-backed release workflow.
6. Validate statistical assumptions against producer-specific traffic dependence and percentile methodology.

### P2 — product and contributor leverage

1. Add full native GuideLLM/inference-perf fixtures for explicitly pinned upstream revisions.
2. Add reviewer-facing HTML/terminal reports that render coverage, rejected evidence, uncertainty, and dependency closure without weakening canonical JSON.
3. Add upstream issue/PR templates for adapter compatibility matrices and evidence-methodology review.
4. Publish a sanitized end-to-end case study showing a real regression caught before deployment and the experiment saved by `INCONCLUSIVE`/counterexample guidance.

## Why product companies need this

Product companies operating LLM features face repeated inference changes: model revisions, quantization, serving-engine upgrades, batching, adapters, replicas, GPU shapes, and routing. Each can affect user latency, reliability, tenant isolation, and unit economics. InferLab provides value in five concrete ways:

1. **Safer releases:** a local policy regression cannot be hidden by better aggregate throughput, and missing data cannot become a pass.
2. **Faster review:** engineers, SREs, product owners, and risk reviewers inspect one deterministic decision closure rather than reconciling dashboards and benchmark formats.
3. **Evidence reuse without false equivalence:** exact runtime/workload identity allows valid evidence to be reused and invalidates it when a material dimension changes.
4. **Vendor-neutral assurance:** benchmark and simulation tools remain replaceable producers while the organization's policy and provenance contract stays stable.
5. **Lower experimentation cost:** `INCONCLUSIVE` identifies the missing measurement instead of encouraging a blind rollout, while verified counterexamples focus the next experiment on the smallest known failure.

The likely adopters are AI platform teams, inference/SRE teams, ML performance engineers, release engineering, FinOps, and organizations that need an auditable pre-deployment change record. The project is most compelling where inference changes are frequent, GPU experiments are expensive, and a latency or fairness regression has real customer impact.

## Final assessment

The repository now demonstrates strong engineering judgment: narrow ownership, honest non-claims, immutable evidence identity, strict trust boundaries, deterministic decisions, statistical uncertainty, reproducible counterexamples, and signed offline review. It is portfolio-grade as a serious systems/reliability project today.

It is not yet production deployment grade. The missing proof is primarily operational and organizational—not a need for more superficial features. The next milestone should be one authorized, sanitized, end-to-end observed AWS case plus protected release governance. That would convert a high-quality pre-alpha core into a credible v0.1 product claim.
