# Uncertainty-aware release gate and counterexamples

InferLab gate v1 answers one narrow question: does the supplied **observed** evidence safely satisfy every mandatory policy for the declared runtime and workload regions? The answer is exactly `PASS`, `BLOCK`, or `INCONCLUSIVE`. A missing measurement is never a pass, and an aggregate improvement never cancels a mandatory violation.

## Decision precedence

The evaluator applies deterministic precedence:

1. any rule with violation probability above the declared maximum produces `BLOCK`;
2. otherwise, any missing, rejected, under-sampled, or confidence-overlapping rule produces `INCONCLUSIVE`; and
3. only when every rule independently passes its conservative bound does the result become `PASS`.

Each finding contains a stable rule ID, region, code, exact metric semantics, evidence digest, observed value, 95% confidence interval, threshold, violation probability, and explanation. The result binds the canonical evaluation digest—including workload coordinates and uncertainty declarations—and includes coverage, every evidence admission/rejection, and a topologically closed graph from runtime/workload/artifact nodes through evidence, evaluation, and policy nodes to the decision.

## Fail-closed evidence admission

An evidence node can satisfy a release rule only when all of the following are true:

- it is `observed` and `complete`;
- its exact runtime is compatible with the target runtime, optionally through an explicit versioned compatibility policy;
- it finished no later than the declared evaluation time and is within the declared age limit;
- its workload digest matches the required region and its concurrency, token, tenant, arrival-rate, and exact fault point lie inside that region;
- the exact metric name, semantic identifier, and unit match the policy contract;
- its sample count reaches the rule's minimum; and
- it carries a supported sampling-uncertainty declaration.

Predicted, derived, asserted, partial, future-dated, stale, runtime-unknown, runtime-incompatible, and out-of-distribution evidence remains visible in admissions but cannot satisfy `PASS`. A fresh compatible observation can still satisfy a region when unrelated rejected evidence is also present; rejected nodes are never silently used.

Evaluation and result documents are strict single JSON values bounded to 8 MiB and 64 nesting levels. V1 additionally caps regions, rules, evidence nodes, aggregate metrics/artifacts, findings, and graph nodes. When many attempts cover one rule, all admissions remain visible but the rule emits only its most conservative accepted finding; rejection findings are deduplicated by stable code. This prevents repeated evidence from amplifying result size without hiding a blocker.

## Metric contracts

Gate v1 recognizes only these directionally fixed contracts:

| Metric | Semantic identifier | Unit | Rule |
| --- | --- | --- | --- |
| TTFT p99 | `request-arrival-to-first-token-v1` | milliseconds | at most |
| TPOT p99 | `output-phase-duration-per-generated-token-v1` | milliseconds | at most |
| ITL p99 | `adjacent-output-token-gap-v1` | milliseconds | at most |
| request goodput | `requests-meeting-declared-slos-v1` | ratio | at least |
| fairness | `weighted-jain-fairness-index-v1` | ratio | at least |
| noisy-neighbour impact | `victim-ttft-relative-degradation-v1` | ratio | at most |
| recovery | `time-to-sustained-slo-recovery-v1` | seconds | at most |
| cost | `amortized-inference-cost-per-million-tokens-v1` | USD | at most |

The validator rejects attempts to reverse a direction or substitute a similar metric definition. TPOT and ITL therefore cannot be pooled. All rules in v1 are mandatory.

## Uncertainty model

Continuous and percentile measurements declare either a bootstrap or sample standard error. The evaluator reports a two-sided 95% normal interval and a one-sided normal-approximation violation probability. Binomial measurements declare exact successes and trials; their interval is the 95% Wilson score interval, and trials must equal the evidence metric's sample count.

`PASS` requires both the conservative confidence bound and the maximum violation-probability rule. If estimated violation risk is acceptable but the stricter interval still crosses the threshold, the result is `INCONCLUSIVE`. This v1 model does not claim distribution-free tail guarantees, independence under autocorrelated traffic, or calibrated simulator uncertainty; those require producer-specific methods and validation evidence.

## Workload and fault support

Coverage regions bound concurrency, prompt/output tokens, tenant count, and arrival rate, and bind one exact workload digest. Fault evidence supports only:

- `replica-loss`: exactly one lost replica for 1–600 seconds; and
- `long-context-spike`: 4,096–262,144 tokens with long-context fraction expressed in one-percentage-point increments.

`none` represents the baseline. Other infrastructure faults are rejected as unsupported. InferLab evaluates supplied fault evidence; it does not mutate cloud or cluster resources, request quotas, or run a chaos campaign.

## Deterministic counterexample minimization

`gate.Minimize` takes a reproducing workload/fault document and a caller-supplied oracle. It deterministically:

1. delta-debugs request subsets;
2. reduces concurrency;
3. reduces replica-loss duration or long-context size/fraction;
4. reduces prompt tokens, output tokens, and arrival offsets; and
5. re-runs the final candidate for the declared number of verification attempts.

Every accepted transformation records before/after digests. The global evaluation budget includes search and reserved verification runs. `search_complete=false` explicitly marks a budget-limited local minimum. An input that does not fail is rejected, oracle errors/cancellation propagate, and a candidate that fails any re-verification is not returned as reproducible. The algorithm claims a minimum only with respect to its declared deterministic transformations and available budget, never a global mathematical minimum.

## CLI

```bash
go run ./cmd/inferlab gate evaluation validate examples/missing-evidence-gate.json
go run ./cmd/inferlab gate evaluate examples/missing-evidence-gate.json
```

`gate evaluate` writes canonical result JSON to stdout and uses exit code `0` for `PASS`, `3` for `BLOCK`, `4` for `INCONCLUSIVE`, `1` for invalid input/execution failure, and `2` for usage errors. The committed example intentionally exits `4`: it demonstrates that a valid policy with no observed evidence cannot pass.

Schemas are published under [`schemas/gate/v1`](../schemas/gate/v1). The Go package also validates and digests results and content-addresses counterexamples for later safety-case closure.
