# massdriver-sdk-go

Go SDK for the [Massdriver](https://www.massdriver.cloud) platform.

> **Beta — breaking changes possible.** The shape of this SDK tracks the
> Massdriver platform API, which is still evolving. Pin to a specific
> commit or tagged release until the API stabilizes; expect occasional
> incompatible changes between minor versions.

## Install

```sh
go get github.com/massdriver-cloud/massdriver-sdk-go/massdriver@latest
```

Requires Go 1.24 or later.

## Quickstart

```go
package main

import (
    "context"
    "fmt"

    "github.com/massdriver-cloud/massdriver-sdk-go/massdriver"
)

func main() {
    c, err := massdriver.NewClient()
    if err != nil {
        panic(err)
    }

    proj, err := c.Projects.Get(context.Background(), "ecommerce")
    if err != nil {
        panic(err)
    }
    fmt.Printf("%s — %d environments\n", proj.Name, len(proj.Environments))
}
```

## Configuration

`massdriver.NewClient()` resolves configuration from (in order of precedence):

| Source | Notes |
| --- | --- |
| Functional options: `WithAPIKey`, `WithOrganizationID`, `WithBaseURL`, `WithProfile` | Highest precedence; useful for explicit credentials in CI or tests. |
| `MASSDRIVER_API_KEY`, `MASSDRIVER_ORGANIZATION_ID`, `MASSDRIVER_URL`, `MASSDRIVER_PROFILE` | Environment variables override the config file. |
| `~/.config/massdriver/config.yaml`, profile selected by `MASSDRIVER_PROFILE` (default `default`) | Created and managed by the [Massdriver CLI](https://github.com/massdriver-cloud/mass). |

```go
// Explicit credentials (e.g. CI):
c, _ := massdriver.NewClient(
    massdriver.WithAPIKey(os.Getenv("DEPLOY_KEY")),
    massdriver.WithOrganizationID("ecommerce"),
)

// Self-hosted instance:
c, _ := massdriver.NewClient(
    massdriver.WithBaseURL("https://massdriver.internal.acme.com"),
)
```

## What's in the box

The top-level `*massdriver.Client` exposes every domain service as a
field. Common entry points:

| Field | Purpose |
| --- | --- |
| `c.Projects` | Top-level project blueprints. |
| `c.Environments` | Deployment contexts within a project. |
| `c.Components` | Components and links inside a project blueprint. |
| `c.Instances` | Deployed bundle instances, their alarms, secrets, and produced resources. |
| `c.Deployments` | Trigger and inspect provisioning runs (incl. live log streaming). |
| `c.Resources` | Provisioned and imported resources, exports, grants. |
| `c.OciRepos` | OCI repositories (CRUD + `oras.Target` for direct artifact access). |
| `c.Bundles` | Read the published bundle catalog. |
| `c.Groups`, `c.Policies` | ABAC groups, members, and policies. |
| `c.AccessTokens`, `c.ServiceAccounts` | Programmatic identities. |
| `c.AuditLogs` | The organization's audit trail (use `Iter` for large queries). |
| `c.Organizations` | Organization metadata + custom-attribute schema. |
| `c.Server`, `c.Viewer`, `c.URLs` | Server metadata, current identity, deep links. |

## Two API surfaces

This repo ships two separate SDK surfaces. Most users only need the
first.

- [`massdriver/platform`](massdriver/platform) — the GraphQL **platform
  API**, accessed through `*massdriver.Client`. Authenticated with
  personal access tokens or service-account keys. This is the public
  SDK for any tooling that talks to Massdriver from outside a
  deployment (CLIs, dashboards, automation, custom integrations).
- [`massdriver/provisioning`](massdriver/provisioning) — REST endpoints
  intended **only** for code running inside a provisioner during a
  deployment run. These endpoints accept deployment-token auth only,
  scoped to the in-flight deployment. Don't import this from
  general-purpose tooling — use `platform` for that.

## Errors

Every wrapper returns errors that callers can classify with `errors.Is`
against sentinels in package `gql`:

```go
proj, err := c.Projects.Get(ctx, id)
switch {
case errors.Is(err, gql.ErrNotFound):       // record doesn't exist
case errors.Is(err, gql.ErrUnauthenticated): // bad/missing credentials
case errors.Is(err, gql.ErrForbidden):       // policy denies the action
case err != nil:                             // anything else
    return err
}
```

For mutations that fail server-side validation, unwrap to
`*gql.MutationFailed` for per-field messages:

```go
if mf, ok := gql.AsMutationFailed(err); ok {
    for _, m := range mf.Messages {
        log.Printf("%s: %s (%s)", m.Field, m.Message, m.Code)
    }
}
```

## Pagination

`List` methods auto-follow cursors and return a slice. For unbounded
queries (e.g. organization-wide audit logs), use `Iter` — a lazy Go 1.23
range-over-func iterator that fetches pages on demand:

```go
for ev, err := range c.AuditLogs.Iter(ctx, auditlogs.ListInput{TimeRangeStart: yesterday}) {
    if err != nil { return err }
    process(ev)
}
```

## Streaming

`c.Deployments.StreamLogs` and `c.Deployments.TailLogs` provide live
deployment logs over an Absinthe websocket subscription. The Service
owns the socket lifetime; cancel `ctx` to stop.

## Testing

Most code that uses the SDK should mock at its own boundary — define a
small interface for the methods your code calls and inject a fake. The
SDK's domain types (`projects.Project`, `deployments.Deployment`, etc.)
are public and constructable.

For tests that need to exercise SDK methods end-to-end, combine
`WithGQLClient` with the [`gql/gqltest`](massdriver/gql/gqltest) mock
GraphQL client:

```go
mock := gqltest.NewClient(
    gqltest.RespondWithData(map[string]any{
        "project": map[string]any{"id": "x", "name": "X"},
    }),
)
c, _ := massdriver.NewClient(
    massdriver.WithGQLClient(mock),
    massdriver.WithOrganizationID("test-org"),
)
proj, _ := c.Projects.Get(ctx, "x")
```

For HTTP-level integration tests, point the SDK at an `httptest`
server with `WithBaseURL`.

### Live-API integration tests

The SDK ships a build-tag-gated integration suite that exercises each
operation against a real Massdriver server. These are intended for
on-demand validation before tagging a release — not for CI on every
commit.

```sh
export MASSDRIVER_API_KEY=mds_...
export MASSDRIVER_ORGANIZATION_ID=<sandbox-org>
# Optional: point at staging
# export MASSDRIVER_URL=https://api.staging.massdriver.cloud

# Run all integration tests:
go test -tags=integration ./massdriver/...

# Run one package's:
go test -tags=integration -v ./massdriver/platform/projects/
```

Conventions:

- Tests live in `*_integration_test.go` with `//go:build integration`.
  They are invisible to a normal `go test ./...` run.
- All fixtures are named with the prefix `inttest-` so orphans from
  a crashed run can be swept manually.
- Tests use `t.Cleanup` for inline teardown — fixtures are removed
  even on assertion failure (but not on panic).
- Running with `-tags=integration` requires credentials. Without them
  the suite fails fast — there is no meaningful integration test we
  can run without a live API.

If you need to clean up orphaned fixtures from a previous crash, list
projects (or other domain entities) in the sandbox and delete those
whose names start with `inttest-`.

## Documentation

Per-package godoc is the primary reference — every service, type, and
input field has an inline doc comment, with runnable `Example` snippets
for the common shapes.

## License

See [LICENSE](LICENSE).
