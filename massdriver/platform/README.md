# platform

GraphQL SDK for the Massdriver platform API.

This is the public surface of the SDK. Use it from any tooling that
talks to Massdriver from outside a deployment — CLIs, dashboards,
automation, custom integrations.

## Auth

Authenticated with a **personal access token** or a **service-account
key**, supplied via `MASSDRIVER_API_KEY` (or `WithAPIKey`).

## Use

Reach these services through the top-level client; don't import the
sub-packages directly except for their input/output types.

```go
c, _ := massdriver.NewClient()
proj, _ := c.Projects.Get(ctx, "ecommerce")
```

See the [root README](../../README.md) for configuration, error
handling, pagination, and streaming.
