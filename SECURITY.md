# Security policy

## Supported versions

InferLab is pre-alpha and does not yet have a supported production release. Security fixes are applied to the `main` branch until the first tagged release. The supported-version table will be updated before v0.1.0.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting for this repository when available: **Security → Report a vulnerability**. If that option is unavailable, email the maintainer at `ngs.gaurav7195@gmail.com` with the subject `InferLab security report`.

Include the affected revision, impact, reproduction steps, and any suggested mitigation. Do not include real prompts, responses, credentials, tenant data, or proprietary traces; use minimal synthetic evidence.

You should receive acknowledgement within 3 business days and an initial assessment within 7 business days. Timelines for a fix and disclosure depend on severity and release status. Please allow a reasonable remediation period before public disclosure.

## Scope priorities

High-priority issues include:

- capture of raw prompt or response content contrary to configuration;
- cross-tenant metadata disclosure or fingerprint reversal;
- production request interference from shadow mode;
- unbounded resource consumption from malicious traces;
- path traversal or arbitrary file overwrite in report generation;
- command/config injection in exporters or integrations;
- authentication, authorization, or secret-handling failures;
- distributed admission errors that can bypass configured limits.

General hardening suggestions without a concrete security impact can be filed as regular issues.
