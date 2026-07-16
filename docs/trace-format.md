# Trace format v1

InferLab traces are UTF-8 JSON Lines files containing scheduling metadata only. Each non-empty line is one independent `inferlab.trace` record. Raw prompts, chat messages, request/response bodies, generated text, embeddings, and tool payloads are outside the schema and are rejected when encountered as JSON fields.

The Go implementation lives in `pkg/trace` and is the normative encoder/decoder for v1. The strict writer contract is also published as [JSON Schema 2020-12](../schemas/trace/v1/record.schema.json) for external tooling.

## Record example

```json
{"schema":"inferlab.trace","schema_version":"1.0","sequence":1,"arrival_offset_ns":1250000,"request_id":"req-0001","tenant_id":"tenant-hmac-sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","model":"qwen-32b","input_tokens":3280,"max_output_tokens":256,"output_tokens":191,"prefix_fingerprint":"prefix-hmac-sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","adapter":"payments-lora-v3","priority":80,"deadline_ms":600,"observed_ttft_ms":418,"observed_tpot_ms":16.2,"selected_endpoint":"worker-7","metadata":{"region":"ap-south-1","traffic.class":"interactive"}}
```

## Fields

| Field | Presence | Unit and semantics |
| --- | --- | --- |
| `schema` | required | Literal `inferlab.trace`. |
| `schema_version` | required | Canonical `MAJOR.MINOR`; the current emitted version is `1.0`. |
| `sequence` | required | One-based stable capture order. |
| `arrival_offset_ns` | required | Non-negative nanoseconds from the capture session's monotonic origin. It is not a wall-clock timestamp. |
| `request_id` | required | Capture-generated correlation ID; do not copy a user-supplied or provider ID without protection. |
| `tenant_id` | required | `tenant-hmac-sha256:` pseudonym generated with an operator-held key. |
| `model` | required | Requested model or validated model alias. |
| `input_tokens` | required | Prompt-token count; greater than zero. |
| `max_output_tokens` | required | Requested output-token ceiling; greater than zero. |
| `output_tokens` | optional | Observed output-token count, no greater than `max_output_tokens`. |
| `prefix_fingerprint` | optional | `prefix-hmac-sha256:` digest over model identity and token IDs. |
| `adapter` | optional | Validated adapter/LoRA identifier. |
| `priority` | required | Integer from 0 through 100. |
| `deadline_ms` | optional | Relative scheduling deadline in milliseconds; zero/absent means unspecified. |
| `observed_ttft_ms` | optional | Finite, non-negative observed time to first token in milliseconds. |
| `observed_tpot_ms` | optional | Finite, non-negative observed time per output token in milliseconds. |
| `selected_endpoint` | optional | Production-selected endpoint identifier. Treat topology names as operational metadata. |
| `metadata` | optional | String map produced by an explicit allowlist. Default capture policy emits none. |

Field names carry their units so downstream tools cannot silently confuse nanoseconds and milliseconds. A future capture manifest will bind the monotonic origin to capture-session provenance without placing wall-clock identity on every record.

## Privacy construction

### Tenant pseudonyms

`Protector.TenantID` computes HMAC-SHA256 with domain `inferlab/tenant/v1`, followed by a zero byte and the UTF-8 tenant identifier. Keys must contain 32 bytes through 4 KiB. The key is copied into the protector, is never written to a trace, and should be randomly generated, access-controlled, and rotated according to operator policy.

HMAC pseudonyms prevent offline dictionary verification by readers who do not have the key. They do not make low-cardinality identifiers anonymous to an operator who retains the key.

### Prefix fingerprints

`Protector.PrefixFingerprint` accepts token IDs rather than raw prompt bytes. It hashes a domain separator, the length-prefixed model identifier, the token count, and each big-endian token ID with HMAC-SHA256. Model separation prevents identical token-number sequences from colliding across tokenizer/model identities.

Fingerprints reveal equality and reuse patterns inside a key domain. Use a distinct key per security boundary and rotate keys when cross-trace linkability is not desired.

### Optional metadata

The metadata policy is fail-closed:

- its default allowlist is empty;
- unapproved keys are dropped and reported to the caller in sorted order;
- malformed or content-shaped input keys are rejected before they can enter the dropped-key audit list;
- input scanning is bounded to 256 entries by default before sorting or copying;
- allowed keys have entry, key-byte, and value-byte limits;
- values must be valid UTF-8 without control characters;
- keys associated with prompts, messages, content, completions, generated text, or bodies cannot be allowlisted.

An allowlist limits accidental collection; operators must still verify that approved values do not contain user data or secrets.

## Canonical encoding

The encoder emits fields in the Go `Record` declaration order, sorts metadata map keys lexicographically through Go's JSON encoder, uses compact JSON, and writes exactly one LF after each record. Identical valid records therefore produce identical bytes with the same supported Go toolchain.

Readers accept LF, CRLF, and a final line without a newline. Empty lines, invalid UTF-8, duplicate object keys, multiple JSON values on one line, non-finite metrics, malformed digests, and sensitive content-field names are rejected. A stream must start at sequence `1`, continue without gaps or duplicates, and use non-decreasing arrival offsets.

## Version compatibility

- Writers emit exactly `1.0` and refuse to label output as a version they do not implement.
- Readers require `schema` to equal `inferlab.trace` and reject unsupported major versions.
- Readers reject unknown fields on the current `1.0` contract. They accept later minor versions in major version 1 and ignore unknown non-sensitive fields after fully validating JSON shape and resource limits.
- The v1.0 JSON Schema is intentionally strict (`additionalProperties: false`) for validating writer output. The Go reader is deliberately more tolerant so it can inspect later minor versions.
- A record decoded from a later minor version cannot be re-emitted by the v1.0 canonical writer. This prevents silently stripping extensions while preserving the newer version label.
- Duplicate keys are invalid rather than using last-value-wins behavior.

Before v1.0 of the InferLab project, schema migrations may accompany minor releases. Every migration must preserve the privacy exclusions above.

## Default resource limits

| Limit | Default |
| --- | ---: |
| Bytes per record | 1 MiB |
| Bytes per trace stream | 4 GiB |
| Records per stream | 1,000,000 |
| Input tokens per record | 4,000,000 |
| Maximum output tokens per record | 1,000,000 |
| Input plus maximum output tokens | 5,000,000 |
| Metadata entries | 32 |
| Metadata key bytes | 64 |
| Metadata value bytes | 512 |
| JSON nesting depth | 16 |

Zero-valued limit configuration selects these defaults rather than disabling a bound. Operators should lower limits for narrower workload envelopes.

The decoder streams records without loading an entire trace into memory. The first malformed or limit-exceeding record terminates the decoder because continuing could hide corruption. Errors expose a typed cause plus the one-based record number and zero-based byte offset. Oversized lines are drained only to their delimiter and are never retained in full.

## Security notes

Treat every trace as untrusted input even when it originated inside the cluster. Store traces with least-privilege access, encrypt them at rest, define retention/deletion, and never publish production traces without a separate privacy review. Trace safety does not imply that model names, adapter names, endpoint topology, timing, or traffic volume are non-sensitive.
