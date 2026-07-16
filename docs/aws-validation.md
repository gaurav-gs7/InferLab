# AWS validation boundary

InferLab's current trusted core is an offline Go CLI and library. It does not require AWS credentials, make AWS API calls, provision infrastructure, execute faults, or deploy an inference server. AWS is a target environment for separately authorized evidence production—not an implicit side effect of validating a change document.

## Reproducible CodeBuild validation

[`buildspec.aws.yml`](../buildspec.aws.yml) is an opt-in AWS CodeBuild recipe for the repository's Linux verification path. It selects Go 1.26, verifies the module and formatting, runs vet/unit/coverage/race/fuzz checks, builds the CLI, reproduces signed `BLOCK` and `INCONCLUSIVE` safety cases, and validates representative public documents. The buildspec itself creates nothing and needs no AWS credentials inside the build; an operator must separately create or select a CodeBuild project and authorize the build cost.

AWS documents buildspec version `0.2`, phase commands, and custom Go runtime selection in the [CodeBuild build specification reference](https://docs.aws.amazon.com/codebuild/latest/userguide/build-spec-ref.html) and [runtime-version reference](https://docs.aws.amazon.com/codebuild/latest/userguide/runtime-versions.html).

## GPU experiment target

The v0.1 inference-change envelope names `g6.xlarge` with one NVIDIA L4. AWS publishes `g6.xlarge` as 4 vCPUs, 16 GiB system memory, one L4, and 24 GB GPU memory in the [EC2 G6 instance specification](https://aws.amazon.com/ec2/instance-types/g6/). Instance-family existence does not prove account quota, capacity at launch time, model fit, driver compatibility, or benchmark correctness.

Before any paid experiment, an operator must provide explicit authorization for the concrete region, resource plan, maximum spend, IAM role, network/storage handling, teardown procedure, and whether On-Demand or Spot interruption semantics are acceptable. A valid InferLab budget is a ceiling consumed by an authorized runner; it is never authorization by itself.

## What has and has not been demonstrated

- Demonstrated locally and in GitHub's Ubuntu CI: the Linux build/test path and offline safety-case workflow.
- Demonstrated with read-only AWS calls on 2026-07-16: configured credentials authenticate, and the EC2 catalog reported `g6.xlarge` offerings in five `us-east-1` Availability Zones.
- Not demonstrated: an AWS CodeBuild execution for this revision, a launched GPU instance, vLLM/model startup, native GuideLLM or inference-perf collection, fault injection, teardown, cost reconciliation, or an observed production-grade `PASS` case.

No quota increase, support case, IAM mutation, resource creation, deployment, or charge was requested or performed during this validation.
