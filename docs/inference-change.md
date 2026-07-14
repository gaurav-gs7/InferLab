# Inference-change contract

An inference-change document is the immutable experiment intent consumed by InferLab. It states what is changing, which workload and policies matter, which faults may be explored, and the maximum money and accelerator time a separately authorized experiment may consume. It is neither a deployment manifest nor authority to perform an external action.

The v1 schema identifier is `inferlab.change` with schema version `1.0`. Documents are strict JSON: unknown fields, trailing values, oversized input, mutable aliases, and unsupported features are rejected rather than ignored.

## Reproducibility boundary

The contract pins:

- baseline and candidate vLLM container images by `sha256` digest;
- model identifiers and immutable revisions;
- quantization, accelerator, instance type, and replica count;
- continuous-batching controls;
- a credential-free `file://` or `s3://` privacy-safe trace URI;
- tenant fairness weights and replay speed;
- latency, fairness, cost, and violation-probability policies;
- bounded fault grids; and
- maximum experiment cost and GPU minutes.

Canonicalization sorts semantically unordered tenant and fault collections before hashing. The resulting `sha256:` digest is the primary intent identifier. It does not prove that an experiment ran correctly; it proves which validated intent an artifact claims to describe. Evidence envelopes must additionally identify source tool and adapter revisions, exact observed runtime and workload signatures, attempts, transformations, and raw artifact digests.

## Supported envelope

Version 0.1 intentionally accepts only:

- engine: `vllm`;
- hardware: NVIDIA L4 on `g6.xlarge`;
- quantization: `none`, `awq`, or `gptq`;
- one or two replicas;
- faults: `replica-loss` and `long-context-spike`; and
- up to 128 weighted tenants.

Unsupported input produces a typed error. Adding a new engine or accelerator requires validation data and an explicit contract change; silently treating different systems as equivalent would invalidate uncertainty claims.

## Safety constraints

Container tags alone are rejected because tags can move. Common mutable model revisions such as `main`, `master`, `latest`, and `HEAD` are rejected. Trace URIs may not contain credentials, query strings, or fragments. Fault points must be positive, strictly increasing, and bounded. Floating-point policy and budget values must be finite.

The budget fields are hard execution ceilings. A runner that has been separately authorized must stop before either ceiling is exceeded and record actual usage. A valid document never authorizes cloud, quota, support, billing, cluster, deployment, or other external-account changes.

## CLI

```bash
inferlab change validate path/to/change.json
inferlab change digest path/to/change.json
```

`validate` prints the document name and canonical digest. `digest` prints only the digest for scripts. Both return exit status `1` for an unreadable or invalid document and `2` for incorrect CLI usage.

The normative machine-readable definition is [`schemas/change/v1/inference-change.schema.json`](../schemas/change/v1/inference-change.schema.json). Go decoding remains the enforcement path and is tested against the published example and schema metadata.

The repository example uses synthetic image digests, model revisions, trace locations, and policy values to demonstrate the format. It validates the contract but is not directly runnable and is not a performance or safety recommendation.
