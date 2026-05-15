// Package deployments provides operations for Massdriver deployments — the
// records of infrastructure provisioning operations against [instance]s.
//
// Each deployment carries a single [Action] (PROVISION, DECOMMISSION, or
// PLAN), the bundle version that ran, the snapshotted params, and the
// lifecycle [Status]. Deployments are immutable once created — modifications
// happen by creating new deployments.
//
// The package surfaces three flavors of deployment creation:
//
//   - [Service.Create] — start a deployment immediately. The standard path.
//   - [Service.Propose] — create a deployment in PROPOSED status that requires
//     approval before running. Use for change-management workflows where
//     an operator must review params before they apply.
//   - [Service.Approve] / [Service.Reject] — release or discard a proposal.
//     [Service.Abort] cancels any pending/approved/running deployment.
//
// Logs are accessed separately via [Service.GetLogs] to keep the standard
// [Service.Get]/[Service.List] payloads small.
//
// Construct a [*Service] with [New] passing the low-level client, or use the
// pre-wired [massdriver.Client.Deployments] field on the top-level SDK client.
package deployments

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/client"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/gql/scalars"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/decode"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/internal/gen"
	"github.com/massdriver-cloud/massdriver-sdk-go/massdriver/platform/types"
)

// Service is the receiver for deployment operations. Construct with [New];
// for the typical case you'll use the [massdriver.Client.Deployments] field.
type Service struct {
	client *client.Client
}

// New returns a [*Service] bound to the given low-level client.
//
// Most callers should use [massdriver.New] instead, which constructs the
// low-level client and pre-wires every service. Use [New] only when you
// need a single service in isolation or for tests with a custom client.
func New(c *client.Client) *Service { return &Service{client: c} }

// Deployment is a Massdriver deployment record — alias of [types.Deployment].
type Deployment = types.Deployment

// Status is a deployment's lifecycle state.
type Status string

const (
	StatusProposed   Status = "PROPOSED"
	StatusRejected   Status = "REJECTED"
	StatusApproved   Status = "APPROVED"
	StatusPending    Status = "PENDING"
	StatusRunning    Status = "RUNNING"
	StatusCompleted  Status = "COMPLETED"
	StatusFailed     Status = "FAILED"
	StatusAborted    Status = "ABORTED"
)

// IsTerminal reports whether the supplied status string is one of the
// terminal deployment states (COMPLETED, FAILED, REJECTED, ABORTED). A
// deployment in a terminal state cannot transition again.
//
// Accepts a string so it works directly on Deployment.Status without a cast:
//
//	if deployments.IsTerminal(dep.Status) { ... }
func IsTerminal(status string) bool {
	switch Status(status) {
	case StatusCompleted, StatusFailed, StatusRejected, StatusAborted:
		return true
	case StatusProposed, StatusApproved, StatusPending, StatusRunning:
		return false
	}
	return false
}

// Action is the infrastructure operation a deployment performs.
type Action string

const (
	// ActionProvision creates or updates infrastructure.
	ActionProvision Action = "PROVISION"
	// ActionDecommission tears down all infrastructure managed by the instance.
	ActionDecommission Action = "DECOMMISSION"
	// ActionPlan generates a dry-run preview without applying changes.
	// Not valid for [Propose]; use [Create] for plans.
	ActionPlan Action = "PLAN"
)

// SortField is the field a [Service.List] result can be ordered by.
type SortField string

const (
	// SortByUpdatedAt — most recently active first. Default when no sort given.
	SortByUpdatedAt SortField = "UPDATED_AT"
	SortByCreatedAt SortField = "CREATED_AT"
	SortByStatus    SortField = "STATUS"
)

// SortOrder is the direction of a sort.
type SortOrder string

const (
	SortAsc  SortOrder = "ASC"
	SortDesc SortOrder = "DESC"
)

// ListInput controls a [Service.List] call. Zero value lists every deployment
// the caller can see, sorted by most recently active first.
type ListInput struct {
	// InstanceID limits results to one instance.
	InstanceID string
	// Status limits results to deployments in that lifecycle state.
	Status Status
	// Action limits results to that infrastructure operation type.
	Action Action
	// OciRepoName limits to deployments of bundles from that repo.
	OciRepoName string

	SortBy    SortField
	SortOrder SortOrder

	PageSize int
}

// CreateInput is the input for [Service.Create]. Params are validated server-side
// against the bundle's params schema and snapshotted onto the deployment.
type CreateInput struct {
	// Action is the operation to perform: PROVISION, DECOMMISSION, or PLAN.
	Action Action
	// Params are the bundle configuration values to apply.
	Params map[string]any
	// Message is an optional commit-message-like description.
	Message string
}

// ProposeInput is the input for [Service.Propose]. Same shape as [CreateInput]
// but Action is restricted to PROVISION or DECOMMISSION (PLAN is not proposable).
type ProposeInput struct {
	// Action must be ActionProvision or ActionDecommission.
	Action Action
	// Params are the bundle configuration values that will apply if the
	// proposal is approved.
	Params map[string]any
	// Message is an optional description. Functions as the proposal's
	// rationale for reviewers.
	Message string
}

// Get retrieves a deployment by ID. Includes the parent instance with slim
// environment/bundle/component refs. Logs are not included; call [Service.GetLogs].
//
// Returns [gql.ErrNotFound] (wrapped, match with [errors.Is]) when no deployment
// with the given ID exists in the configured organization.
func (s *Service) Get(ctx context.Context, id string) (*Deployment, error) {
	resp, err := gen.GetDeployment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("get deployment %s: %w", id, err))
	}
	if resp.Deployment.Id == "" {
		return nil, fmt.Errorf("get deployment %s: %w", id, gql.ErrNotFound)
	}
	return toDeployment(resp.Deployment)
}

// GetLogs returns the deployment's logs to-date as a single concatenated
// string, oldest-first. Each batch is appended in order; if a batch's
// message doesn't end in a newline a separator is inserted so adjacent
// batches don't fuse onto one line.
//
// For live tailing — receiving new batches as they arrive — use
// [Service.StreamLogs] instead. A common pattern is to call GetLogs first
// to print the backfill, then open a stream for whatever the deployment
// emits next.
func (s *Service) GetLogs(ctx context.Context, id string) (string, error) {
	resp, err := gen.GetDeploymentLogs(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return "", gql.ClassifyError(fmt.Errorf("get logs for deployment %s: %w", id, err))
	}
	var sb strings.Builder
	for _, l := range resp.Deployment.Logs {
		sb.WriteString(l.Message)
		if !strings.HasSuffix(l.Message, "\n") {
			sb.WriteByte('\n')
		}
	}
	return sb.String(), nil
}

// Iter returns a [iter.Seq2] over deployments matching the supplied
// filters, fetching pages lazily on demand. Use this for large or
// potentially-unbounded result sets where buffering every match in
// memory ([Service.List]) is impractical.
//
// The yielded error is non-nil exactly when the underlying transport
// or decode failed. Stop iterating after observing one — subsequent
// items are not produced.
//
// Cancellation is via ctx; break out of the range loop to stop
// requesting further pages.
//
// Example:
//
//	for dep, err := range svc.Iter(ctx, deployments.ListInput{InstanceID: "ecomm-prod-database"}) {
//	    if err != nil { return err }
//	    process(dep)
//	}
func (s *Service) Iter(ctx context.Context, input ListInput) iter.Seq2[Deployment, error] {
	filter := buildListFilter(input)
	sort := buildListSort(input)

	return func(yield func(Deployment, error) bool) {
		var cursor *scalars.Cursor
		if input.PageSize > 0 {
			cursor = &scalars.Cursor{Limit: input.PageSize}
		}
		for {
			resp, err := gen.ListDeployments(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, cursor)
			if err != nil {
				yield(Deployment{}, gql.ClassifyError(fmt.Errorf("list deployments: %w", err)))
				return
			}
			for _, item := range resp.Deployments.Items {
				d, derr := toDeployment(item)
				if derr != nil {
					yield(Deployment{}, derr)
					return
				}
				if !yield(*d, nil) {
					return
				}
			}
			next := resp.Deployments.Cursor.Next
			if next == "" {
				return
			}
			cursor = &scalars.Cursor{Next: next}
			if input.PageSize > 0 {
				cursor.Limit = input.PageSize
			}
		}
	}
}

// List returns deployments matching the supplied filters, following
// pagination cursors automatically and buffering every match into a
// single slice. Returned deployments carry a slim instance ref
// (id+name only) and no params/logs — call [Service.Get] /
// [Service.GetLogs] for those.
//
// For large result sets — anything where the full match could be tens
// of thousands of rows — prefer [Service.Iter], which yields one
// deployment at a time. Cancel ctx to stop early.
func (s *Service) List(ctx context.Context, input ListInput) ([]Deployment, error) {
	filter := buildListFilter(input)
	sort := buildListSort(input)

	var (
		out    []Deployment
		cursor *scalars.Cursor
	)
	if input.PageSize > 0 {
		cursor = &scalars.Cursor{Limit: input.PageSize}
	}

	for {
		resp, err := gen.ListDeployments(ctx, s.client.GQLv2, s.client.Config.OrganizationID, filter, sort, cursor)
		if err != nil {
			return nil, gql.ClassifyError(fmt.Errorf("list deployments: %w", err))
		}
		for _, item := range resp.Deployments.Items {
			d, derr := toDeployment(item)
			if derr != nil {
				return nil, fmt.Errorf("decode deployment: %w", derr)
			}
			out = append(out, *d)
		}
		next := resp.Deployments.Cursor.Next
		if next == "" {
			break
		}
		cursor = &scalars.Cursor{Next: next}
		if input.PageSize > 0 {
			cursor.Limit = input.PageSize
		}
	}
	return out, nil
}

// Create starts a new deployment for the named instance. The deployment
// enters the lifecycle at PENDING and transitions to RUNNING when execution
// begins.
func (s *Service) Create(ctx context.Context, instanceID string, input CreateInput) (*Deployment, error) {
	resp, err := gen.CreateDeployment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, instanceID, gen.CreateDeploymentInput{
		Action:  gen.DeploymentAction(input.Action),
		Params:  input.Params,
		Message: input.Message,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("create deployment for instance %s: %w", instanceID, err))
	}
	if err := gql.CheckMutation("create deployment", resp.CreateDeployment.Successful, resp.CreateDeployment.Messages); err != nil {
		return nil, err
	}
	return toDeployment(resp.CreateDeployment.Result)
}

// Propose creates a deployment in PROPOSED status that requires approval
// before running. Approve with [Service.Approve], reject with [Service.Reject].
//
// PLAN is not a valid action here — plans are non-destructive previews and
// don't need an approval gate. Server returns a validation error if you
// pass ActionPlan.
func (s *Service) Propose(ctx context.Context, instanceID string, input ProposeInput) (*Deployment, error) {
	resp, err := gen.ProposeDeployment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, instanceID, gen.ProposeDeploymentInput{
		Action:  gen.ProposeDeploymentAction(input.Action),
		Params:  input.Params,
		Message: input.Message,
	})
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("propose deployment for instance %s: %w", instanceID, err))
	}
	if err := gql.CheckMutation("propose deployment", resp.ProposeDeployment.Successful, resp.ProposeDeployment.Messages); err != nil {
		return nil, err
	}
	return toDeployment(resp.ProposeDeployment.Result)
}

// Approve releases a PROPOSED deployment into the run queue. The deployment
// transitions to APPROVED and runs as soon as nothing else is running on the
// instance. Only valid for deployments currently in PROPOSED status.
func (s *Service) Approve(ctx context.Context, id string) (*Deployment, error) {
	resp, err := gen.ApproveDeployment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("approve deployment %s: %w", id, err))
	}
	if err := gql.CheckMutation("approve deployment", resp.ApproveDeployment.Successful, resp.ApproveDeployment.Messages); err != nil {
		return nil, err
	}
	return toDeployment(resp.ApproveDeployment.Result)
}

// Reject discards a PROPOSED deployment permanently. Transition to REJECTED
// is terminal — rejected deployments never run. Only valid for deployments
// currently in PROPOSED status.
func (s *Service) Reject(ctx context.Context, id string) (*Deployment, error) {
	resp, err := gen.RejectDeployment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("reject deployment %s: %w", id, err))
	}
	if err := gql.CheckMutation("reject deployment", resp.RejectDeployment.Successful, resp.RejectDeployment.Messages); err != nil {
		return nil, err
	}
	return toDeployment(resp.RejectDeployment.Result)
}

// Abort cancels a PENDING, APPROVED, or RUNNING deployment. The deployment
// transitions to ABORTED. Note: a running deployment aborted mid-flight
// leaves any partial infrastructure changes the provisioner had applied in
// place. Use [Service.Reject] to discard a PROPOSED deployment.
func (s *Service) Abort(ctx context.Context, id string) (*Deployment, error) {
	resp, err := gen.AbortDeployment(ctx, s.client.GQLv2, s.client.Config.OrganizationID, id)
	if err != nil {
		return nil, gql.ClassifyError(fmt.Errorf("abort deployment %s: %w", id, err))
	}
	if err := gql.CheckMutation("abort deployment", resp.AbortDeployment.Successful, resp.AbortDeployment.Messages); err != nil {
		return nil, err
	}
	return toDeployment(resp.AbortDeployment.Result)
}

func toDeployment(v any) (*Deployment, error) {
	d := Deployment{}
	if err := decode.Decode(v, &d); err != nil {
		return nil, fmt.Errorf("decode deployment: %w", err)
	}
	return &d, nil
}

func buildListFilter(input ListInput) *gen.DeploymentsFilter {
	filter := &gen.DeploymentsFilter{}
	set := false
	if input.InstanceID != "" {
		filter.InstanceId = &gen.IdFilter{Eq: input.InstanceID}
		set = true
	}
	if input.Status != "" {
		filter.Status = &gen.DeploymentStatusFilter{Eq: gen.DeploymentStatus(input.Status)}
		set = true
	}
	if input.Action != "" {
		filter.Action = &gen.DeploymentActionFilter{Eq: gen.DeploymentAction(input.Action)}
		set = true
	}
	if input.OciRepoName != "" {
		filter.OciRepoName = &gen.OciRepoNameFilter{Eq: input.OciRepoName}
		set = true
	}
	if !set {
		return nil
	}
	return filter
}

func buildListSort(input ListInput) *gen.DeploymentsSort {
	if input.SortBy == "" && input.SortOrder == "" {
		return nil
	}
	field := gen.DeploymentsSortFieldUpdatedAt
	switch input.SortBy {
	case SortByCreatedAt:
		field = gen.DeploymentsSortFieldCreatedAt
	case SortByStatus:
		field = gen.DeploymentsSortFieldStatus
	case SortByUpdatedAt:
		// already the default
	}
	order := gen.SortOrderDesc
	if input.SortOrder == SortAsc {
		order = gen.SortOrderAsc
	}
	return &gen.DeploymentsSort{Field: field, Order: order}
}
