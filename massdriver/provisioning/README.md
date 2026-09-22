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

## Usage

```go
import (
    "github.com/massdriver-cloud/massdriver-sdk-go/massdriver/provisioning"
    "github.com/massdriver-cloud/massdriver-sdk-go/massdriver/provisioning/resources"
)

pc, err := provisioning.NewClient()
if err != nil {
    log.Fatal(err)
}
res, err := pc.Resources.CreateResource(ctx, &resources.ResourceInput{
    Field:   "bucket",
    Name:    "Asset store",
    Payload: map[string]any{"arn": "arn:aws:s3:::assets"},
})
```

`Field` names the output declared under `resources` in the producing
bundle's `massdriver.yaml`. The server resolves the resource type from
it, so the request never names a type. The returned `resources.Resource`
carries the resolved type and version.

`provisioning.NewClient()` resolves deployment-token credentials from
`MASSDRIVER_DEPLOYMENT_ID` + `MASSDRIVER_TOKEN`, which the platform
injects into the provisioner container at deployment time.

## Packages

- `deployments` — report deployment status back to the platform.
- `resources` — read and write resources owned by the in-flight
  deployment.
