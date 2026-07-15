package gate

type metricContract struct {
	semantics string
	unit      string
	direction Direction
}

const (
	MetricTTFTP99              = "ttft-p99"
	MetricTPOTP99              = "tpot-p99"
	MetricITLP99               = "itl-p99"
	MetricRequestGoodput       = "request-goodput"
	MetricFairnessIndex        = "fairness-index"
	MetricNoisyNeighborImpact  = "noisy-neighbor-impact"
	MetricRecoverySeconds      = "recovery-seconds"
	MetricCostPerMillionTokens = "cost-per-million-tokens"
)

var metricCatalog = map[string]metricContract{
	MetricTTFTP99:              {semantics: "request-arrival-to-first-token-v1", unit: "milliseconds", direction: DirectionAtMost},
	MetricTPOTP99:              {semantics: "output-phase-duration-per-generated-token-v1", unit: "milliseconds", direction: DirectionAtMost},
	MetricITLP99:               {semantics: "adjacent-output-token-gap-v1", unit: "milliseconds", direction: DirectionAtMost},
	MetricRequestGoodput:       {semantics: "requests-meeting-declared-slos-v1", unit: "ratio", direction: DirectionAtLeast},
	MetricFairnessIndex:        {semantics: "weighted-jain-fairness-index-v1", unit: "ratio", direction: DirectionAtLeast},
	MetricNoisyNeighborImpact:  {semantics: "victim-ttft-relative-degradation-v1", unit: "ratio", direction: DirectionAtMost},
	MetricRecoverySeconds:      {semantics: "time-to-sustained-slo-recovery-v1", unit: "seconds", direction: DirectionAtMost},
	MetricCostPerMillionTokens: {semantics: "amortized-inference-cost-per-million-tokens-v1", unit: "usd", direction: DirectionAtMost},
}
