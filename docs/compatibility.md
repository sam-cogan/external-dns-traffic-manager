# Supported compatibility

`compatibility.json` is the machine-readable source of supported versions. CI
checks it against the Go and Kubernetes dependencies, container base-image
digests, contract-test lockfiles, deployment examples, and generated deployment
manifests. Updates to a supported version are compatibility changes and must
pass the complete matrix before release.

| Component | Tested support |
|---|---|
| Go | 1.27.4 toolchain; container builder pinned to the same patch |
| ExternalDNS | v0.23.0 and v0.24.0; webhook protocol v1; examples use v0.24.0 |
| Kubernetes client libraries | `k8s.io/api`, `apimachinery`, and `client-go` v0.37.1 |
| Kubernetes API and kubectl | 1.35.8, 1.36.4, and 1.37.0; upstream Kind images pinned by digest and matching kubectl clients |
| Azure SDK | `azcore` v1.23.3, `azidentity` v1.15.0, `armtrafficmanager` v1.3.0 |
| DNSEndpoint | `externaldns.k8s.io/v1alpha1`, installed using `apiextensions.k8s.io/v1` |

ExternalDNS contract tests execute each tagged upstream webhook client against
the production HTTP handlers. Kubernetes API validation uses disposable local
API servers. Neither is a live AKS, Azure DNS, or Traffic Manager qualification.
AKS version availability depends on region and changes over time; check the
[AKS Kubernetes release calendar](https://learn.microsoft.com/azure/aks/supported-kubernetes-versions)
before deployment. The matrix lists upstream Kubernetes API versions, not a
promise that every AKS region offers them.

ExternalDNS v0.14.0 and v0.20.0 are retired. All examples use the supported
default v0.24.0. The sample service annotations use
`external-dns.kubernetes.io/`, the current ExternalDNS default prefix; its
webhook annotation processor transmits provider-specific properties as
`webhook/traffic-manager-*`.

## Updating and deprecation

Dependabot opens grouped weekly updates for Kubernetes libraries, Azure SDK
modules, other Go dependencies, ExternalDNS contract clients, GitHub Actions,
and Docker base images. Major updates remain separate, the number of open
updates is capped, and updates are never auto-merged. ExternalDNS container
images are pinned in examples and generated manifests rather than a Dockerfile;
`compatibility.json` and the manifest consistency check catch image drift.

The maintainers review the support matrix at least quarterly and when upstream
ends support, a security advisory affects a supported component, or a required
dependency changes incompatibly. New support is advertised only after every
matrix job passes. Give users a migration notice for a removal at least one
release before removing the old version, unless a critical security issue makes
continued support unsafe. Update this document, the machine-readable matrix,
contract locks, images, and examples together.

## Security and license exceptions

CI fails closed on HIGH or CRITICAL dependency and image findings, source
scanner failures, unknown dependency licenses, and licenses outside the
allowlist in `.github/workflows/ci.yml`. Scanner/database outages are failures,
not clean scans. Advisory databases are live inputs, so vulnerability results
may change even when source and dependency locks are unchanged.

Do not add blanket ignores, unbounded exceptions, or automatic suppressions.
For a verified false positive or a temporary remediation exception, submit a
separate reviewed change that names the exact tool and finding, affected
component/version, evidence and rationale, tracking issue, accountable
approver, and a short expiry (no more than 30 days). Keep the exception scoped
to that finding, retain the underlying scan, test that unrelated findings still
fail, and remove or renew the exception before expiry. License-policy changes
also require review of the actual license and redistribution obligations.

## Branch protection

The required GitHub Actions check is **CI required**. It succeeds only when the
quality/risk-coverage, both ExternalDNS contract, all Kubernetes manifest/API,
security/license, and container/image-scan gates succeed. It runs on pull
requests and pushes without path filters so the required status is not skipped.
The `main` branch should require this check, require the branch to be up to
date, include administrators, and prohibit force pushes and deletion. The
repository protection settings are enforced separately from this checked-in
policy.
