package types

import "time"

// EventAction is the lifecycle change carried by every [Event] — what
// happened to the affected resource. Values mirror the platform's
// `EventAction` enum.
type EventAction string

const (
	// EventCreated is the action emitted when a resource is created.
	EventCreated EventAction = "CREATED"
	// EventUpdated is the action emitted when a resource is modified
	// (configuration, status, or metadata changed).
	EventUpdated EventAction = "UPDATED"
	// EventDeleted is the action emitted when a resource is permanently
	// removed.
	EventDeleted EventAction = "DELETED"
)

// Event is the marker interface satisfied by every event variant yielded by
// a `StreamEvents` subscription. The concrete variants are [ProjectEvent],
// [EnvironmentEvent], [EnvironmentDefaultEvent], [InstanceEvent],
// [ComponentEvent], [LinkEvent], [ConnectionEvent], [AlarmEvent],
// [OciRepoEvent], [BundleEvent], and [DeploymentEvent].
//
// Type-assert to the concrete variant to read the affected resource:
//
//	for ev := range events {
//	    switch e := ev.(type) {
//	    case *types.InstanceEvent:
//	        fmt.Println(e.Action, e.Instance.Name)
//	    case *types.AlarmEvent:
//	        fmt.Println(e.Action, e.Alarm.DisplayName)
//	    }
//	}
//
// Every variant embeds [EventCommon] for the shared `Action` and
// `Timestamp` fields, reachable via field promotion.
type Event interface {
	isEvent()
}

// EventCommon carries the fields every event variant shares — embedded
// into each concrete variant. Promoted via embedding so call sites read
// e.g. `ev.(*InstanceEvent).Action` rather than going through an
// accessor method.
type EventCommon struct {
	Action    EventAction `json:"action"`
	Timestamp time.Time   `json:"timestamp"`
}

func (EventCommon) isEvent() {}

// ProjectEvent is a lifecycle event for a [Project] — fires on
// create, update, and delete. Delivered via the project,
// organization subscriptions.
type ProjectEvent struct {
	EventCommon
	Project Project `json:"project"`
}

// EnvironmentEvent is a lifecycle event for an [Environment]. Creation
// events arrive on the parent project's subscription; updates and
// deletes arrive on the environment's own subscription.
type EnvironmentEvent struct {
	EventCommon
	Environment Environment `json:"environment"`
}

// EnvironmentDefaultEvent is a lifecycle event for an
// [EnvironmentDefault] — emitted CREATED when a default is set and
// DELETED when it is cleared. Defaults are paginated under the parent
// environment, so refetch that page on receipt rather than relying on
// cache merging.
type EnvironmentDefaultEvent struct {
	EventCommon
	EnvironmentDefault EnvironmentDefault `json:"environmentDefault"`
}

// InstanceEvent is a lifecycle event for an [Instance] — fires on
// create, configuration change, deployment, and delete.
type InstanceEvent struct {
	EventCommon
	Instance Instance `json:"instance"`
}

// ComponentEvent is a lifecycle event for a [Component] — fires when
// a component is added, renamed/re-tagged, moved on the canvas, or
// removed from a project's blueprint.
type ComponentEvent struct {
	EventCommon
	Component Component `json:"component"`
}

// LinkEvent is a lifecycle event for a blueprint [Link] — fires when
// two components are linked or unlinked. Links have no mutable body, so
// only CREATED and DELETED actions are emitted.
type LinkEvent struct {
	EventCommon
	Link Link `json:"link"`
}

// ConnectionEvent is a lifecycle event for a [Connection] — the
// runtime realization of a [Link] within an environment. Fires when
// the wire is established or torn down.
type ConnectionEvent struct {
	EventCommon
	Connection Connection `json:"connection"`
}

// AlarmEvent is a lifecycle event for a cloud-metric [Alarm] attached
// to an instance. Fires on register / reconfigure / remove, and on
// firing-state transitions (OK → ALARM, etc.) which surface as
// UPDATED with the latest state on `Alarm.CurrentState`.
type AlarmEvent struct {
	EventCommon
	Alarm Alarm `json:"alarm"`
}

// OciRepoEvent is a lifecycle event for an [OciRepo]. The registry is
// immutable, so only CREATED is emitted today — the first time a
// bundle is published under a new repository name.
type OciRepoEvent struct {
	EventCommon
	OciRepo OciRepo `json:"ociRepo"`
}

// BundleEvent is a lifecycle event for a published [Bundle] version.
// Bundle versions are immutable, so only CREATED is emitted today.
type BundleEvent struct {
	EventCommon
	Bundle Bundle `json:"bundle"`
}

// DeploymentEvent is a lifecycle event for a [Deployment] — emitted
// on create (CREATED) and each status transition (UPDATED). Log
// content is not carried; subscribe via `StreamLogs` for that.
type DeploymentEvent struct {
	EventCommon
	Deployment Deployment `json:"deployment"`
}
