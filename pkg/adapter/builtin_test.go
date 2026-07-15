package adapter

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/gaurav-gs7/InferLab/pkg/evidence"
)

func TestBuiltinsConformAndNormalizeDeterministically(t *testing.T) {
	t.Parallel()
	for _, implementation := range Builtins() {
		implementation := implementation
		t.Run(implementation.Capabilities().Adapter.Name, func(t *testing.T) {
			t.Parallel()
			input := testInput(implementation.Capabilities().Adapter.Name)
			if err := CheckConformance(implementation, []Input{input}); err != nil {
				t.Fatal(err)
			}
			first, err := implementation.Normalize(input)
			if err != nil {
				t.Fatal(err)
			}
			second, err := implementation.Normalize(input)
			if err != nil {
				t.Fatal(err)
			}
			firstDigest, err := Digest(first)
			if err != nil {
				t.Fatal(err)
			}
			secondDigest, err := Digest(second)
			if err != nil {
				t.Fatal(err)
			}
			if firstDigest != secondDigest {
				t.Fatalf("digests differ: %s != %s", firstDigest, secondDigest)
			}
			if first.InputDigest != inputDigest(input.Report) || first.Envelope.Artifacts[0].Digest != first.InputDigest {
				t.Fatal("normalized report is not bound to the exact producer bytes")
			}
			if len(first.Originals) != len(first.Envelope.Metrics) {
				t.Fatal("normalization lost original values")
			}
		})
	}
}

func TestCommittedProducerFixturesConform(t *testing.T) {
	t.Parallel()
	fixtures := map[string]string{
		GuideLLMAdapterName:      "guidellm-v0.6.0-report.json",
		InferencePerfAdapterName: "inference-perf-v0.1.0-report.json",
		AnalyticalAdapterName:    "analytical-prediction-v1-report.json",
	}
	for name, fileName := range fixtures {
		name, fileName := name, fileName
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			raw, err := os.ReadFile(filepath.Join("testdata", fileName))
			if err != nil {
				t.Fatal(err)
			}
			input := testInput(name)
			input.Report = raw
			implementation, _ := Builtin(name)
			if err := CheckConformance(implementation, []Input{input}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestUnitNormalizationAndSemanticSeparation(t *testing.T) {
	t.Parallel()
	guide, _ := Builtin(GuideLLMAdapterName)
	guideReport, err := guide.Normalize(testInput(GuideLLMAdapterName))
	if err != nil {
		t.Fatal(err)
	}
	wantGuide := map[string]float64{
		"ttft-p99": 1000, "tpot-p99": 2000, "request-goodput": 0.97, "prompt-tokens-mean": 4, "cost-per-million-tokens": 2.5,
	}
	for _, metric := range guideReport.Envelope.Metrics {
		if metric.Value != wantGuide[metric.Name] {
			t.Errorf("%s = %v, want %v", metric.Name, metric.Value, wantGuide[metric.Name])
		}
	}

	perf, _ := Builtin(InferencePerfAdapterName)
	perfReport, err := perf.Normalize(testInput(InferencePerfAdapterName))
	if err != nil {
		t.Fatal(err)
	}
	var tpotSemantics, itlSemantics string
	for _, metric := range guideReport.Envelope.Metrics {
		if metric.Name == "tpot-p99" {
			tpotSemantics = metric.Semantics
		}
	}
	for _, metric := range perfReport.Envelope.Metrics {
		if metric.Name == "itl-p99" {
			itlSemantics = metric.Semantics
		}
	}
	if tpotSemantics == "" || itlSemantics == "" || tpotSemantics == itlSemantics {
		t.Fatalf("TPOT semantics %q and ITL semantics %q must remain distinct", tpotSemantics, itlSemantics)
	}
}

func TestPredictedAdapterCannotRelabelObservation(t *testing.T) {
	t.Parallel()
	implementation, _ := Builtin(AnalyticalAdapterName)
	input := testInput(AnalyticalAdapterName)
	input.Classification = evidence.ClassObserved
	input.Runtime = testRuntime(evidence.OriginObserved)
	_, err := implementation.Normalize(input)
	if !errors.Is(err, ErrClassification) {
		t.Fatalf("error = %v, want classification violation", err)
	}
}

func TestNormalizationRejectsSemanticDriftAndIncompleteClaims(t *testing.T) {
	t.Parallel()
	implementation, _ := Builtin(GuideLLMAdapterName)
	tests := []struct {
		name    string
		mutate  func(*Input, *producerReport)
		wantErr error
	}{
		{name: "unknown definition", mutate: func(_ *Input, report *producerReport) { report.Metrics[0].Definition = "server-handler-time-v1" }, wantErr: ErrUnsupportedMetric},
		{name: "ambiguous unit", mutate: func(_ *Input, report *producerReport) { report.Metrics[0].Unit = "ms" }, wantErr: ErrUnsupportedMetric},
		{name: "missing complete metric", mutate: func(_ *Input, report *producerReport) { report.Metrics = report.Metrics[:1] }, wantErr: ErrInvalidInput},
		{name: "duplicate metric", mutate: func(_ *Input, report *producerReport) { report.Metrics[1] = report.Metrics[0] }, wantErr: ErrInvalidInput},
		{name: "wrong producer", mutate: func(input *Input, _ *producerReport) { input.Producer.ToolVersion = "0.7.0" }, wantErr: ErrUnsupportedProducer},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			input := testInput(GuideLLMAdapterName)
			var raw producerReport
			if err := json.Unmarshal(input.Report, &raw); err != nil {
				t.Fatal(err)
			}
			tt.mutate(&input, &raw)
			encoded, err := json.Marshal(raw)
			if err == nil {
				input.Report = encoded
			}
			_, gotErr := implementation.Normalize(input)
			if !errors.Is(gotErr, tt.wantErr) {
				t.Fatalf("error = %v, want %v", gotErr, tt.wantErr)
			}
		})
	}
}

func TestCanonicalJSONIgnoresProjectionOrderButNotRawArtifactIdentity(t *testing.T) {
	t.Parallel()
	implementation, _ := Builtin(GuideLLMAdapterName)
	report, err := implementation.Normalize(testInput(GuideLLMAdapterName))
	if err != nil {
		t.Fatal(err)
	}
	reordered := report
	reordered.Originals = slices.Clone(report.Originals)
	reordered.Mappings = slices.Clone(report.Mappings)
	reordered.Envelope.Metrics = slices.Clone(report.Envelope.Metrics)
	slices.Reverse(reordered.Originals)
	slices.Reverse(reordered.Mappings)
	slices.Reverse(reordered.Envelope.Metrics)
	first, err := CanonicalJSON(report)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalJSON(reordered)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("canonical projection depends on metric order")
	}
}

func TestValidateNormalizedReportDetectsTampering(t *testing.T) {
	t.Parallel()
	implementation, _ := Builtin(GuideLLMAdapterName)
	report, err := implementation.Normalize(testInput(GuideLLMAdapterName))
	if err != nil {
		t.Fatal(err)
	}
	report.Mappings[0].NormalizedValue++
	if err := ValidateNormalizedReport(report); !errors.Is(err, ErrInvalidReport) {
		t.Fatalf("error = %v, want invalid report", err)
	}
}
