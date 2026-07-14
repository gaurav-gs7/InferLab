package trace

import "strings"

func validRecord() Record {
	ttft := 418.0
	tpot := 16.2
	return Record{
		Schema:            Schema,
		SchemaVersion:     CurrentSchemaVersion,
		Sequence:          1,
		ArrivalOffsetNS:   1_250_000,
		RequestID:         "req-0001",
		TenantID:          tenantDigestPrefix + strings.Repeat("a", 64),
		Model:             "qwen-32b",
		InputTokens:       3_280,
		MaxOutputTokens:   256,
		OutputTokens:      191,
		PrefixFingerprint: prefixDigestPrefix + strings.Repeat("b", 64),
		Adapter:           "payments-lora-v3",
		Priority:          80,
		DeadlineMS:        600,
		ObservedTTFTMS:    &ttft,
		ObservedTPOTMS:    &tpot,
		SelectedEndpoint:  "worker-7",
		Metadata: map[string]string{
			"traffic.class": "interactive",
			"region":        "ap-south-1",
		},
	}
}
