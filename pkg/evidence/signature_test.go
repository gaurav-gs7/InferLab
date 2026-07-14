package evidence

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestRuntimeSignatureMaterialMutationsInvalidate(t *testing.T) {
	t.Parallel()

	mutations := []struct {
		dimension Dimension
		mutate    func(*RuntimeSignature)
	}{
		{DimensionModelID, func(s *RuntimeSignature) { s.Model.ID += "-candidate" }},
		{DimensionModelRevision, func(s *RuntimeSignature) { s.Model.Revision = strings.Repeat("a", 40) }},
		{DimensionTokenizerID, func(s *RuntimeSignature) { s.Model.TokenizerID += "-candidate" }},
		{DimensionTokenizerRevision, func(s *RuntimeSignature) { s.Model.TokenizerRevision = strings.Repeat("b", 40) }},
		{DimensionQuantization, func(s *RuntimeSignature) { s.Model.Quantization = "gptq" }},
		{DimensionQuantizationConfig, func(s *RuntimeSignature) { s.Model.QuantizationConfigDigest = "sha256:" + strings.Repeat("c", 64) }},
		{DimensionEngineName, func(s *RuntimeSignature) { s.Engine.Name = "vllm-candidate" }},
		{DimensionEngineRevision, func(s *RuntimeSignature) { s.Engine.Revision = strings.Repeat("d", 40) }},
		{DimensionContainerImage, func(s *RuntimeSignature) {
			s.Engine.ContainerImage = "ghcr.io/vllm-project/vllm-openai@sha256:" + strings.Repeat("e", 64)
		}},
		{DimensionCUDAVersion, func(s *RuntimeSignature) { s.Platform.CUDAVersion = "12.9" }},
		{DimensionDriverVersion, func(s *RuntimeSignature) { s.Platform.DriverVersion = "571.0" }},
		{DimensionGPUSKU, func(s *RuntimeSignature) { s.Platform.GPUSKU = "nvidia-l40s" }},
		{DimensionGPUCount, func(s *RuntimeSignature) { s.Platform.GPUCount = 2 }},
		{DimensionTopology, func(s *RuntimeSignature) { s.Platform.Topology = "pcie-p2p" }},
		{DimensionSchedulerName, func(s *RuntimeSignature) { s.Scheduler.Name = "vllm-v2" }},
		{DimensionSchedulerConfig, func(s *RuntimeSignature) { s.Scheduler.ConfigDigest = "sha256:" + strings.Repeat("f", 64) }},
		{DimensionKernels, func(s *RuntimeSignature) { s.Kernels[0].Version = "2.7.5" }},
	}

	for _, mutation := range mutations {
		mutation := mutation
		t.Run(string(mutation.dimension), func(t *testing.T) {
			t.Parallel()
			baseline := validRuntimeSignature(OriginDeclared)
			candidate := validRuntimeSignature(OriginObserved)
			mutation.mutate(&candidate)
			result, err := CompareRuntimeSignatures(baseline, candidate, nil)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != CompatibilityMismatch {
				t.Fatalf("status = %q, want %q", result.Status, CompatibilityMismatch)
			}
			if len(result.Differences) != 1 || result.Differences[0] != mutation.dimension {
				t.Fatalf("differences = %v, want [%s]", result.Differences, mutation.dimension)
			}
		})
	}
}

func TestValidateRuntimeSignature(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		mutate func(*RuntimeSignature)
	}{
		{name: "schema", mutate: func(s *RuntimeSignature) { s.Schema = "other" }},
		{name: "version", mutate: func(s *RuntimeSignature) { s.SchemaVersion = "2.0" }},
		{name: "origin", mutate: func(s *RuntimeSignature) { s.Origin = "guessed" }},
		{name: "model id control", mutate: func(s *RuntimeSignature) { s.Model.ID = "model\n" }},
		{name: "tokenizer revision", mutate: func(s *RuntimeSignature) { s.Model.TokenizerRevision = "latest" }},
		{name: "quantization name", mutate: func(s *RuntimeSignature) { s.Model.Quantization = "AWQ" }},
		{name: "quantization digest", mutate: func(s *RuntimeSignature) { s.Model.QuantizationConfigDigest = "sha256:nope" }},
		{name: "engine name", mutate: func(s *RuntimeSignature) { s.Engine.Name = "vLLM" }},
		{name: "engine revision", mutate: func(s *RuntimeSignature) { s.Engine.Revision = "main" }},
		{name: "gpu count", mutate: func(s *RuntimeSignature) { s.Platform.GPUCount = maxGPUCount + 1 }},
		{name: "mutable cuda", mutate: func(s *RuntimeSignature) { s.Platform.CUDAVersion = "latest" }},
		{name: "mutable driver", mutate: func(s *RuntimeSignature) { s.Platform.DriverVersion = "unknown" }},
		{name: "scheduler name", mutate: func(s *RuntimeSignature) { s.Scheduler.Name = "vLLM" }},
		{name: "scheduler digest", mutate: func(s *RuntimeSignature) { s.Scheduler.ConfigDigest = "bad" }},
		{name: "too many kernels", mutate: func(s *RuntimeSignature) { s.Kernels = make([]KernelIdentity, maxKernels+1) }},
		{name: "nil kernels", mutate: func(s *RuntimeSignature) { s.Kernels = nil }},
		{name: "kernel name", mutate: func(s *RuntimeSignature) { s.Kernels[0].Name = "Flash Attention" }},
		{name: "kernel version", mutate: func(s *RuntimeSignature) { s.Kernels[0].Version = "" }},
		{name: "mutable kernel version", mutate: func(s *RuntimeSignature) { s.Kernels[0].Version = "main" }},
		{name: "kernel digest", mutate: func(s *RuntimeSignature) { s.Kernels[0].ConfigDigest = "bad" }},
		{name: "duplicate kernel", mutate: func(s *RuntimeSignature) { s.Kernels = append(s.Kernels, s.Kernels[0]) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			signature := validRuntimeSignature(OriginObserved)
			tt.mutate(&signature)
			if err := ValidateRuntimeSignature(signature); err == nil {
				t.Fatal("ValidateRuntimeSignature() accepted invalid input")
			}
		})
	}
}

func TestCompareRuntimeSignatures(t *testing.T) {
	t.Parallel()

	declared := validRuntimeSignature(OriginDeclared)
	observed := validRuntimeSignature(OriginObserved)
	exact, err := CompareRuntimeSignatures(declared, observed, nil)
	if err != nil {
		t.Fatal(err)
	}
	if exact.Status != CompatibilityExact {
		t.Fatalf("status = %q, want exact", exact.Status)
	}

	observed.Platform.DriverVersion = "571.0"
	policy := &CompatibilityPolicy{
		Name:              "driver-patch-compatible",
		Version:           "1.0.0",
		IgnoredDimensions: []Dimension{DimensionDriverVersion},
	}
	compatible, err := CompareRuntimeSignatures(declared, observed, policy)
	if err != nil {
		t.Fatal(err)
	}
	if compatible.Status != CompatibilityByPolicy || len(compatible.IgnoredDifferences) != 1 {
		t.Fatalf("comparison = %#v, want compatible-by-policy", compatible)
	}
}

func TestUnknownIdentityNeverMatches(t *testing.T) {
	t.Parallel()
	left := validRuntimeSignature(OriginDeclared)
	right := validRuntimeSignature(OriginObserved)
	right.Platform.DriverVersion = ""
	policy := &CompatibilityPolicy{
		Name:              "driver-patch-compatible",
		Version:           "1.0",
		IgnoredDimensions: []Dimension{DimensionDriverVersion},
	}
	result, err := CompareRuntimeSignatures(left, right, policy)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != CompatibilityUnknown || len(result.UnknownDimensions) != 1 || result.UnknownDimensions[0] != DimensionDriverVersion {
		t.Fatalf("comparison = %#v, want unknown driver", result)
	}
}

func TestCompatibilityPolicyValidation(t *testing.T) {
	t.Parallel()
	left := validRuntimeSignature(OriginDeclared)
	right := validRuntimeSignature(OriginObserved)
	tests := []CompatibilityPolicy{
		{Name: "Invalid", Version: "1.0"},
		{Name: "valid", Version: "latest"},
		{Name: "valid", Version: "1.0", IgnoredDimensions: []Dimension{"invented"}},
		{Name: "valid", Version: "1.0", IgnoredDimensions: []Dimension{DimensionGPUCount, DimensionGPUCount}},
		{Name: "valid", Version: "1.0", IgnoredDimensions: []Dimension{DimensionTopology, DimensionGPUCount}},
	}
	for _, policy := range tests {
		policy := policy
		if _, err := CompareRuntimeSignatures(left, right, &policy); !errors.Is(err, ErrInvalidPolicy) {
			t.Fatalf("policy %#v error = %v, want %v", policy, err, ErrInvalidPolicy)
		}
	}
}

func TestCompatibilityMismatchPrecedesUnknown(t *testing.T) {
	t.Parallel()
	left := validRuntimeSignature(OriginDeclared)
	right := validRuntimeSignature(OriginObserved)
	right.Platform.DriverVersion = ""
	right.Platform.GPUSKU = "nvidia-l40s"
	result, err := CompareRuntimeSignatures(left, right, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != CompatibilityMismatch || len(result.UnknownDimensions) != 1 {
		t.Fatalf("comparison = %#v, want mismatch with recorded unknown", result)
	}
}

func TestRuntimeSignatureCanonicalDigest(t *testing.T) {
	t.Parallel()
	first := validRuntimeSignature(OriginObserved)
	first.Kernels = append(first.Kernels, KernelIdentity{Name: "xformers", Version: "0.0.30", ConfigDigest: "sha256:" + strings.Repeat("b", 64)})
	second := first
	second.Kernels = []KernelIdentity{first.Kernels[1], first.Kernels[0]}
	firstDigest, err := RuntimeSignatureDigest(first)
	if err != nil {
		t.Fatal(err)
	}
	secondDigest, err := RuntimeSignatureDigest(second)
	if err != nil {
		t.Fatal(err)
	}
	if firstDigest != secondDigest {
		t.Fatalf("equivalent kernel order produced %q and %q", firstDigest, secondDigest)
	}
}

func TestValidateRuntimeSignatureRejectsMutableIdentity(t *testing.T) {
	t.Parallel()
	signature := validRuntimeSignature(OriginObserved)
	signature.Engine.ContainerImage = "vllm:latest"
	if err := ValidateRuntimeSignature(signature); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidSignature)
	}
	signature = validRuntimeSignature(OriginObserved)
	signature.Model.Revision = "main"
	if err := ValidateRuntimeSignature(signature); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("error = %v, want %v", err, ErrInvalidSignature)
	}
}

func TestDecodeRuntimeSignatureRejectsUntrustedInput(t *testing.T) {
	t.Parallel()
	encoded, err := json.Marshal(validRuntimeSignature(OriginObserved))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		reader  *bytes.Reader
		wantErr error
	}{
		{name: "trailing value", reader: bytes.NewReader(append(encoded, []byte("\n{}")...)), wantErr: ErrInvalidSignature},
		{name: "too large", reader: bytes.NewReader(bytes.Repeat([]byte("x"), MaxDocumentBytes+1)), wantErr: ErrDocumentTooLarge},
		{name: "empty", reader: bytes.NewReader(nil), wantErr: ErrInvalidSignature},
		{name: "duplicate field", reader: bytes.NewReader([]byte(`{"schema":"inferlab.runtime-signature","schema":"other"}`)), wantErr: ErrDuplicateField},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := DecodeRuntimeSignature(tt.reader)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
	if _, err := DecodeRuntimeSignature(nil); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("nil reader error = %v, want %v", err, ErrInvalidSignature)
	}
}
