package adapter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

const (
	GuideLLMAdapterName      = "guidellm-fixture-v1"
	InferencePerfAdapterName = "inference-perf-fixture-v1"
	AnalyticalAdapterName    = "predicted-json-v1"
)

// Mapping defines one exact, reviewable producer-to-evidence conversion.
type Mapping struct {
	Capability MetricCapability
	Scale      float64
	Offset     float64
}

type mappedAdapter struct {
	capabilities Capabilities
	mappings     map[string]Mapping
}

type producerReport struct {
	Schema  string           `json:"schema"`
	RunID   string           `json:"run_id"`
	Metrics []OriginalMetric `json:"metrics"`
}

// NewPredictedAdapter creates a predicted-only adapter for an immutable
// analytical producer contract. It cannot emit observed evidence.
func NewPredictedAdapter(identity AdapterIdentity, producer ProducerIdentity, mappings []Mapping) (Adapter, error) {
	return newMappedAdapter(identity, producer, []evidence.Classification{evidence.ClassPredicted}, mappings)
}

func newMappedAdapter(identity AdapterIdentity, producer ProducerIdentity, classifications []evidence.Classification, mappings []Mapping) (Adapter, error) {
	capabilities := Capabilities{
		Schema:          ProtocolSchema,
		SchemaVersion:   CurrentVersion,
		Adapter:         identity,
		Producer:        producer,
		Classifications: slices.Clone(classifications),
		Metrics:         make([]MetricCapability, 0, len(mappings)),
	}
	indexed := make(map[string]Mapping, len(mappings))
	for _, mapping := range mappings {
		if math.IsNaN(mapping.Scale) || math.IsInf(mapping.Scale, 0) || mapping.Scale == 0 || math.IsNaN(mapping.Offset) || math.IsInf(mapping.Offset, 0) {
			return nil, fmt.Errorf("%w: mapping scale and offset must be finite and scale non-zero", ErrInvalidCapabilities)
		}
		capabilities.Metrics = append(capabilities.Metrics, mapping.Capability)
		key := sourceMetricKey(mapping.Capability.SourceName, mapping.Capability.SourceDefinition, mapping.Capability.SourceUnit)
		indexed[key] = mapping
	}
	if err := ValidateCapabilities(capabilities); err != nil {
		return nil, err
	}
	return &mappedAdapter{capabilities: capabilities, mappings: indexed}, nil
}

func (adapter *mappedAdapter) Capabilities() Capabilities {
	capabilities := adapter.capabilities
	capabilities.Classifications = slices.Clone(adapter.capabilities.Classifications)
	capabilities.Metrics = slices.Clone(adapter.capabilities.Metrics)
	return capabilities
}

func (adapter *mappedAdapter) Normalize(input Input) (NormalizedReport, error) {
	if err := ValidateInput(input); err != nil {
		return NormalizedReport{}, err
	}
	if input.Producer != adapter.capabilities.Producer {
		return NormalizedReport{}, fmt.Errorf("%w: got %s@%s/%s", ErrUnsupportedProducer, input.Producer.Tool, input.Producer.ToolVersion, input.Producer.ReportSchema)
	}
	if !slices.Contains(adapter.capabilities.Classifications, input.Classification) {
		return NormalizedReport{}, fmt.Errorf("%w: adapter %s cannot emit %s", ErrClassification, adapter.capabilities.Adapter.Name, input.Classification)
	}

	raw, err := decodeStrict[producerReport](bytes.NewReader(input.Report), MaxInputBytes, ErrInvalidInput)
	if err != nil {
		return NormalizedReport{}, err
	}
	if raw.Schema != input.Producer.ReportSchema || !namePattern.MatchString(raw.RunID) {
		return NormalizedReport{}, fmt.Errorf("%w: report schema or run_id is invalid", ErrInvalidInput)
	}
	if len(raw.Metrics) == 0 || len(raw.Metrics) > len(adapter.mappings) {
		return NormalizedReport{}, fmt.Errorf("%w: report contains an invalid metric count", ErrInvalidInput)
	}
	if input.Completeness == evidence.CompletenessComplete && len(raw.Metrics) != len(adapter.mappings) {
		return NormalizedReport{}, fmt.Errorf("%w: complete report requires all %d declared metrics", ErrInvalidInput, len(adapter.mappings))
	}

	originals := make([]OriginalMetric, 0, len(raw.Metrics))
	mappingRecords := make([]MappingRecord, 0, len(raw.Metrics))
	metrics := make([]evidence.Metric, 0, len(raw.Metrics))
	seen := make(map[string]struct{}, len(raw.Metrics))
	for i, original := range raw.Metrics {
		if !namePattern.MatchString(original.Name) || !namePattern.MatchString(original.Definition) || original.Unit == "" ||
			math.IsNaN(original.Value) || math.IsInf(original.Value, 0) || original.SampleCount == 0 {
			return NormalizedReport{}, fmt.Errorf("%w: metrics[%d] is invalid", ErrInvalidInput, i)
		}
		key := sourceMetricKey(original.Name, original.Definition, original.Unit)
		if _, exists := seen[key]; exists {
			return NormalizedReport{}, fmt.Errorf("%w: duplicate metric %q", ErrInvalidInput, original.Name)
		}
		seen[key] = struct{}{}
		mapping, supported := adapter.mappings[key]
		if !supported {
			return NormalizedReport{}, fmt.Errorf("%w: %s/%s/%s", ErrUnsupportedMetric, original.Name, original.Definition, original.Unit)
		}
		normalized := original.Value*mapping.Scale + mapping.Offset
		if math.IsNaN(normalized) || math.IsInf(normalized, 0) {
			return NormalizedReport{}, fmt.Errorf("%w: conversion for %q is non-finite", ErrInvalidInput, original.Name)
		}
		if (mapping.Capability.NormalizedUnit == "ratio" || mapping.Capability.NormalizedUnit == "probability") && (normalized < 0 || normalized > 1) {
			return NormalizedReport{}, fmt.Errorf("%w: normalized %q is outside [0,1]", ErrInvalidInput, original.Name)
		}
		originals = append(originals, original)
		mappingRecords = append(mappingRecords, MappingRecord{
			SourceName:       original.Name,
			SourceDefinition: original.Definition,
			SourceValue:      original.Value,
			SourceUnit:       original.Unit,
			NormalizedName:   mapping.Capability.NormalizedName,
			Semantics:        mapping.Capability.Semantics,
			NormalizedValue:  normalized,
			NormalizedUnit:   mapping.Capability.NormalizedUnit,
			Scale:            mapping.Scale,
			Offset:           mapping.Offset,
		})
		metrics = append(metrics, evidence.Metric{
			Name:        mapping.Capability.NormalizedName,
			Semantics:   mapping.Capability.Semantics,
			Value:       normalized,
			Unit:        mapping.Capability.NormalizedUnit,
			SampleCount: original.SampleCount,
		})
	}

	digest := inputDigest(input.Report)
	envelope := evidence.NewEnvelope()
	envelope.Name = input.Name
	envelope.Classification = input.Classification
	envelope.Completeness = input.Completeness
	envelope.Source = evidence.Source{
		Tool:            input.Producer.Tool,
		ToolVersion:     input.Producer.ToolVersion,
		ReportSchema:    input.Producer.ReportSchema,
		Adapter:         adapter.capabilities.Adapter.Name,
		AdapterRevision: adapter.capabilities.Adapter.Revision,
	}
	envelope.Runtime = input.Runtime
	envelope.WorkloadDigest = input.WorkloadDigest
	envelope.Attempt = input.Attempt
	envelope.StartedAt = input.StartedAt
	envelope.FinishedAt = input.FinishedAt
	envelope.Metrics = metrics
	envelope.Artifacts = []evidence.Artifact{{Name: "raw-report", MediaType: "application/json", Digest: digest}}

	report := NormalizedReport{
		Schema:        NormalizedReportSchema,
		SchemaVersion: CurrentVersion,
		Adapter:       adapter.capabilities.Adapter,
		InputDigest:   digest,
		Originals:     originals,
		Mappings:      mappingRecords,
		Envelope:      envelope,
	}
	if err := ValidateNormalizedReport(report); err != nil {
		return NormalizedReport{}, err
	}
	return report, nil
}

func sourceMetricKey(name, definition, unit string) string {
	return strings.Join([]string{name, definition, unit}, "\x00")
}

func metric(sourceName, sourceDefinition, sourceUnit, normalizedName, semantics, normalizedUnit string, scale float64) Mapping {
	return Mapping{
		Capability: MetricCapability{
			SourceName:       sourceName,
			SourceDefinition: sourceDefinition,
			SourceUnit:       sourceUnit,
			NormalizedName:   normalizedName,
			Semantics:        semantics,
			NormalizedUnit:   normalizedUnit,
		},
		Scale: scale,
	}
}

func guideLLMAdapter() Adapter {
	adapter, err := newMappedAdapter(
		AdapterIdentity{Name: GuideLLMAdapterName, Revision: "b7de4b1c89b41f7c58bba4f9c91596edc69799381b61020a0ba6ed281a1689f8"},
		ProducerIdentity{Tool: "guidellm", ToolVersion: "0.6.0", ReportSchema: "guidellm-benchmark-v1"},
		[]evidence.Classification{evidence.ClassObserved},
		[]Mapping{
			metric("time-to-first-token-p99", "request-arrival-to-first-token-v1", "seconds", "ttft-p99", "request-arrival-to-first-token-v1", "milliseconds", 1000),
			metric("time-per-output-token-p99", "output-phase-duration-per-generated-token-v1", "seconds", "tpot-p99", "output-phase-duration-per-generated-token-v1", "milliseconds", 1000),
			metric("request-goodput", "requests-meeting-declared-slos-v1", "percent", "request-goodput", "requests-meeting-declared-slos-v1", "ratio", 0.01),
			metric("prompt-tokens-mean", "request-input-token-count-mean-v1", "tokens", "prompt-tokens-mean", "request-input-token-count-mean-v1", "tokens", 1),
			metric("cost-per-1k-tokens", "amortized-inference-cost-per-1k-tokens-v1", "usd-per-1k-tokens", "cost-per-million-tokens", "amortized-inference-cost-per-million-tokens-v1", "usd", 1000),
		},
	)
	if err != nil {
		panic(err)
	}
	return adapter
}

func inferencePerfAdapter() Adapter {
	adapter, err := newMappedAdapter(
		AdapterIdentity{Name: InferencePerfAdapterName, Revision: "2162e5f8de1f9274c464cf3f5ed56b1ebb69b348b0625bce392f2930419c9cc6"},
		ProducerIdentity{Tool: "inference-perf", ToolVersion: "0.1.0", ReportSchema: "inference-perf-results-v1"},
		[]evidence.Classification{evidence.ClassObserved},
		[]Mapping{
			metric("ttft-p99", "request-arrival-to-first-token-v1", "microseconds", "ttft-p99", "request-arrival-to-first-token-v1", "milliseconds", 0.001),
			metric("inter-token-latency-p99", "adjacent-output-token-gap-v1", "microseconds", "itl-p99", "adjacent-output-token-gap-v1", "milliseconds", 0.001),
			metric("output-token-throughput", "successful-output-tokens-per-wall-second-v1", "tokens-per-second", "output-token-throughput", "successful-output-tokens-per-wall-second-v1", "tokens_per_second", 1),
			metric("offered-concurrency", "simultaneous-client-workers-v1", "requests", "offered-concurrency", "simultaneous-client-workers-v1", "count", 1),
		},
	)
	if err != nil {
		panic(err)
	}
	return adapter
}

func analyticalAdapter() Adapter {
	adapter, err := NewPredictedAdapter(
		AdapterIdentity{Name: AnalyticalAdapterName, Revision: "972c632cd46b4aa746965591f286ff3d20a916ff4f8de7d9ab108a91860181a0"},
		ProducerIdentity{Tool: "analytical-fixture", ToolVersion: "1.0.0", ReportSchema: "inferlab-predicted-metrics-v1"},
		[]Mapping{
			metric("ttft-p99", "request-arrival-to-first-token-v1", "milliseconds", "ttft-p99", "request-arrival-to-first-token-v1", "milliseconds", 1),
			metric("request-goodput", "requests-meeting-declared-slos-v1", "probability", "request-goodput", "requests-meeting-declared-slos-v1", "ratio", 1),
			metric("slo-violation-probability", "posterior-policy-violation-probability-v1", "percent", "slo-violation-probability", "posterior-policy-violation-probability-v1", "probability", 0.01),
		},
	)
	if err != nil {
		panic(err)
	}
	return adapter
}

// Builtins returns a fresh, deterministically ordered built-in registry.
func Builtins() []Adapter {
	return []Adapter{analyticalAdapter(), guideLLMAdapter(), inferencePerfAdapter()}
}

func Builtin(name string) (Adapter, bool) {
	for _, adapter := range Builtins() {
		if adapter.Capabilities().Adapter.Name == name {
			return adapter, true
		}
	}
	return nil, false
}

// MarshalCapabilities returns stable, indented capabilities for CLI display.
func MarshalCapabilities(capabilities Capabilities) ([]byte, error) {
	if err := ValidateCapabilities(capabilities); err != nil {
		return nil, err
	}
	return json.MarshalIndent(capabilities, "", "  ")
}
