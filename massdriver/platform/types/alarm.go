package types

import "time"

// Alarm is a cloud metric alarm attached to an [Instance]. State updates
// arrive via webhook from AWS CloudWatch, Azure Monitor, GCP Cloud
// Monitoring, or Prometheus Alertmanager.
//
// Field availability varies by provider — for example, AWS and Azure
// populate Statistic/Threshold/Period/ComparisonOperator while Alertmanager
// often leaves those zero. CurrentState is the most recent state transition
// reported for the alarm; nil when no state has been recorded yet.
type Alarm struct {
	ID                 string       `json:"id" mapstructure:"id"`
	DisplayName        string       `json:"displayName" mapstructure:"displayName"`
	CloudResourceID    string       `json:"cloudResourceId" mapstructure:"cloudResourceId"`
	ComparisonOperator string       `json:"comparisonOperator,omitempty" mapstructure:"comparisonOperator"`
	Threshold          float64      `json:"threshold,omitempty" mapstructure:"threshold"`
	Period             int          `json:"period,omitempty" mapstructure:"period"`
	Metric             *AlarmMetric `json:"metric,omitempty" mapstructure:"metric,omitempty"`
	CurrentState       *AlarmState  `json:"currentState,omitempty" mapstructure:"currentState,omitempty"`
	CreatedAt          time.Time    `json:"createdAt,omitzero" mapstructure:"createdAt"`
	UpdatedAt          time.Time    `json:"updatedAt,omitzero" mapstructure:"updatedAt"`
}

// AlarmMetric describes the cloud metric an alarm evaluates. Field
// availability depends on the provider — expect partial population.
type AlarmMetric struct {
	Namespace  string                 `json:"namespace,omitempty" mapstructure:"namespace"`
	Name       string                 `json:"name,omitempty" mapstructure:"name"`
	Statistic  string                 `json:"statistic,omitempty" mapstructure:"statistic"`
	Region     string                 `json:"region,omitempty" mapstructure:"region"`
	Dimensions []AlarmMetricDimension `json:"dimensions,omitempty" mapstructure:"dimensions"`
}

// AlarmMetricDimension is a key/value pair identifying the cloud resource a
// metric applies to.
type AlarmMetricDimension struct {
	Name  string `json:"name" mapstructure:"name"`
	Value string `json:"value" mapstructure:"value"`
}

// AlarmState is a single state transition reported for an [Alarm].
type AlarmState struct {
	ID         string    `json:"id" mapstructure:"id"`
	Status     string    `json:"status" mapstructure:"status"`
	Message    string    `json:"message,omitempty" mapstructure:"message"`
	OccurredAt time.Time `json:"occurredAt,omitzero" mapstructure:"occurredAt"`
}
