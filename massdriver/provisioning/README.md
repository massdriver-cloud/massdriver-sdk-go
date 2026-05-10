# provisioning

REST clients for endpoints invoked **from inside a provisioner during a
deployment run**.

These endpoints are publicly reachable but intentionally narrow: they
accept **deployment token** auth only, scoped to the in-flight
deployment. They are consumed by code that executes inside a
provisioner container — the `xo` CLI baked into provisioner images,
the Massdriver Terraform provider, and similar deploy-time tooling.

## Don't use this from general-purpose code

If your code is **not** running inside a provisioner during a
deployment, you want [`massdriver/platform`](../platform) instead —
the GraphQL SDK that accepts personal access tokens and
service-account keys.

## Packages

- `artifacts` — publish/read deployment artifacts.
- `deployments` — report deployment status back to the platform.
- `resources` — read and write resources owned by the in-flight
  deployment.
