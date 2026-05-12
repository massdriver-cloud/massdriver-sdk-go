package types

// CostSummary is the cloud-provider cost rollup attached to a [Project],
// [Environment], or [Instance].
type CostSummary struct {
	LastMonth      CostSample `json:"lastMonth" mapstructure:"lastMonth"`
	MonthlyAverage CostSample `json:"monthlyAverage" mapstructure:"monthlyAverage"`
	LastDay        CostSample `json:"lastDay" mapstructure:"lastDay"`
	DailyAverage   CostSample `json:"dailyAverage" mapstructure:"dailyAverage"`
}

// CostSample is one cost data point. Amount and Currency are nilable:
// nil means Massdriver has no billing data for the requested period.
// When set, Currency is an ISO 4217 code (e.g. "USD").
type CostSample struct {
	Amount   *float64 `json:"amount,omitempty" mapstructure:"amount,omitempty"`
	Currency *string  `json:"currency,omitempty" mapstructure:"currency,omitempty"`
}
