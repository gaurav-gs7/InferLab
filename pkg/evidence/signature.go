package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	maxIdentifierBytes = 256
	maxKernels         = 64
	maxGPUCount        = 1024
)

var (
	revisionPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
	digestPattern   = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	imagePattern    = regexp.MustCompile(`^[^[:space:]@]+@sha256:[0-9a-f]{64}$`)
	namePattern     = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,126}[a-z0-9])?$`)
	mutableAliases  = map[string]struct{}{"canary": {}, "dev": {}, "edge": {}, "head": {}, "latest": {}, "main": {}, "master": {}, "nightly": {}, "snapshot": {}, "trunk": {}, "unknown": {}, "unstable": {}}
)

// Dimension names one material runtime identity field.
type Dimension string

const (
	DimensionModelID            Dimension = "model.id"
	DimensionModelRevision      Dimension = "model.revision"
	DimensionTokenizerID        Dimension = "model.tokenizer_id"
	DimensionTokenizerRevision  Dimension = "model.tokenizer_revision"
	DimensionQuantization       Dimension = "model.quantization"
	DimensionQuantizationConfig Dimension = "model.quantization_config_digest"
	DimensionEngineName         Dimension = "engine.name"
	DimensionEngineRevision     Dimension = "engine.revision"
	DimensionContainerImage     Dimension = "engine.container_image"
	DimensionCUDAVersion        Dimension = "platform.cuda_version"
	DimensionDriverVersion      Dimension = "platform.driver_version"
	DimensionGPUSKU             Dimension = "platform.gpu_sku"
	DimensionGPUCount           Dimension = "platform.gpu_count"
	DimensionTopology           Dimension = "platform.topology"
	DimensionSchedulerName      Dimension = "scheduler.name"
	DimensionSchedulerConfig    Dimension = "scheduler.config_digest"
	DimensionKernels            Dimension = "kernels"
)

var materialDimensions = []Dimension{
	DimensionModelID,
	DimensionModelRevision,
	DimensionTokenizerID,
	DimensionTokenizerRevision,
	DimensionQuantization,
	DimensionQuantizationConfig,
	DimensionEngineName,
	DimensionEngineRevision,
	DimensionContainerImage,
	DimensionCUDAVersion,
	DimensionDriverVersion,
	DimensionGPUSKU,
	DimensionGPUCount,
	DimensionTopology,
	DimensionSchedulerName,
	DimensionSchedulerConfig,
	DimensionKernels,
}

// ValidateRuntimeSignature validates syntax while permitting explicitly
// incomplete identity. Use UnknownDimensions to determine admissibility.
func ValidateRuntimeSignature(signature RuntimeSignature) error {
	if signature.Schema != SignatureSchema {
		return fmt.Errorf("%w: %q", ErrUnsupportedSchema, signature.Schema)
	}
	if signature.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("%w: %q", ErrUnsupportedVersion, signature.SchemaVersion)
	}
	if signature.Origin != OriginDeclared && signature.Origin != OriginObserved {
		return fmt.Errorf("%w: origin must be declared or observed", ErrInvalidSignature)
	}
	if err := validateOptionalIdentifier("model.id", signature.Model.ID); err != nil {
		return err
	}
	if err := validateOptionalRevision("model.revision", signature.Model.Revision); err != nil {
		return err
	}
	if err := validateOptionalIdentifier("model.tokenizer_id", signature.Model.TokenizerID); err != nil {
		return err
	}
	if err := validateOptionalRevision("model.tokenizer_revision", signature.Model.TokenizerRevision); err != nil {
		return err
	}
	if err := validateOptionalName("model.quantization", signature.Model.Quantization); err != nil {
		return err
	}
	if err := validateOptionalDigest("model.quantization_config_digest", signature.Model.QuantizationConfigDigest); err != nil {
		return err
	}
	if err := validateOptionalName("engine.name", signature.Engine.Name); err != nil {
		return err
	}
	if err := validateOptionalRevision("engine.revision", signature.Engine.Revision); err != nil {
		return err
	}
	if signature.Engine.ContainerImage != "" && !imagePattern.MatchString(signature.Engine.ContainerImage) {
		return fmt.Errorf("%w: engine.container_image must contain an immutable sha256 digest", ErrInvalidSignature)
	}
	if err := validateOptionalIdentifier("platform.cuda_version", signature.Platform.CUDAVersion); err != nil {
		return err
	}
	if isMutableAlias(signature.Platform.CUDAVersion) {
		return fmt.Errorf("%w: platform.cuda_version must identify a pinned version", ErrInvalidSignature)
	}
	if err := validateOptionalIdentifier("platform.driver_version", signature.Platform.DriverVersion); err != nil {
		return err
	}
	if isMutableAlias(signature.Platform.DriverVersion) {
		return fmt.Errorf("%w: platform.driver_version must identify a pinned version", ErrInvalidSignature)
	}
	if err := validateOptionalIdentifier("platform.gpu_sku", signature.Platform.GPUSKU); err != nil {
		return err
	}
	if signature.Platform.GPUCount > maxGPUCount {
		return fmt.Errorf("%w: platform.gpu_count exceeds %d", ErrInvalidSignature, maxGPUCount)
	}
	if err := validateOptionalIdentifier("platform.topology", signature.Platform.Topology); err != nil {
		return err
	}
	if err := validateOptionalName("scheduler.name", signature.Scheduler.Name); err != nil {
		return err
	}
	if err := validateOptionalDigest("scheduler.config_digest", signature.Scheduler.ConfigDigest); err != nil {
		return err
	}
	if signature.Kernels == nil {
		return fmt.Errorf("%w: kernels must be an array; use an empty array for unknown", ErrInvalidSignature)
	}
	if len(signature.Kernels) > maxKernels {
		return fmt.Errorf("%w: kernels has %d entries, maximum is %d", ErrInvalidSignature, len(signature.Kernels), maxKernels)
	}
	seen := make(map[string]struct{}, len(signature.Kernels))
	for i, kernel := range signature.Kernels {
		if !namePattern.MatchString(kernel.Name) {
			return fmt.Errorf("%w: kernels[%d].name is invalid", ErrInvalidSignature, i)
		}
		if err := validateRequiredIdentifier(fmt.Sprintf("kernels[%d].version", i), kernel.Version, ErrInvalidSignature); err != nil {
			return err
		}
		if isMutableAlias(kernel.Version) {
			return fmt.Errorf("%w: kernels[%d].version must identify a pinned version", ErrInvalidSignature, i)
		}
		if !digestPattern.MatchString(kernel.ConfigDigest) {
			return fmt.Errorf("%w: kernels[%d].config_digest must be a sha256 digest", ErrInvalidSignature, i)
		}
		if _, exists := seen[kernel.Name]; exists {
			return fmt.Errorf("%w: kernels repeats name %q", ErrInvalidSignature, kernel.Name)
		}
		seen[kernel.Name] = struct{}{}
	}
	return nil
}

// UnknownDimensions returns sorted material dimensions not identified by the signature.
func UnknownDimensions(signature RuntimeSignature) ([]Dimension, error) {
	if err := ValidateRuntimeSignature(signature); err != nil {
		return nil, err
	}
	values, err := dimensionValues(signature)
	if err != nil {
		return nil, err
	}
	unknown := make([]Dimension, 0)
	for _, dimension := range materialDimensions {
		if values[dimension] == "" {
			unknown = append(unknown, dimension)
		}
	}
	return unknown, nil
}

// CanonicalRuntimeSignatureJSON returns the stable runtime identity representation.
func CanonicalRuntimeSignatureJSON(signature RuntimeSignature) ([]byte, error) {
	if err := ValidateRuntimeSignature(signature); err != nil {
		return nil, err
	}
	canonical := canonicalSignature(signature)
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("encode canonical runtime signature: %w", err)
	}
	return encoded, nil
}

// RuntimeSignatureDigest returns a sha256-prefixed canonical identity digest.
func RuntimeSignatureDigest(signature RuntimeSignature) (string, error) {
	canonical, err := CanonicalRuntimeSignatureJSON(signature)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(canonical)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func canonicalSignature(signature RuntimeSignature) RuntimeSignature {
	canonical := signature
	canonical.Kernels = slices.Clone(signature.Kernels)
	slices.SortFunc(canonical.Kernels, func(a, b KernelIdentity) int {
		return strings.Compare(a.Name, b.Name)
	})
	return canonical
}

func dimensionValues(signature RuntimeSignature) (map[Dimension]string, error) {
	kernels := canonicalSignature(signature).Kernels
	kernelValue := ""
	if len(kernels) > 0 {
		encoded, err := json.Marshal(kernels)
		if err != nil {
			return nil, fmt.Errorf("encode kernels: %w", err)
		}
		kernelValue = string(encoded)
	}
	gpuCount := ""
	if signature.Platform.GPUCount > 0 {
		gpuCount = fmt.Sprint(signature.Platform.GPUCount)
	}
	return map[Dimension]string{
		DimensionModelID:            signature.Model.ID,
		DimensionModelRevision:      signature.Model.Revision,
		DimensionTokenizerID:        signature.Model.TokenizerID,
		DimensionTokenizerRevision:  signature.Model.TokenizerRevision,
		DimensionQuantization:       signature.Model.Quantization,
		DimensionQuantizationConfig: signature.Model.QuantizationConfigDigest,
		DimensionEngineName:         signature.Engine.Name,
		DimensionEngineRevision:     signature.Engine.Revision,
		DimensionContainerImage:     signature.Engine.ContainerImage,
		DimensionCUDAVersion:        signature.Platform.CUDAVersion,
		DimensionDriverVersion:      signature.Platform.DriverVersion,
		DimensionGPUSKU:             signature.Platform.GPUSKU,
		DimensionGPUCount:           gpuCount,
		DimensionTopology:           signature.Platform.Topology,
		DimensionSchedulerName:      signature.Scheduler.Name,
		DimensionSchedulerConfig:    signature.Scheduler.ConfigDigest,
		DimensionKernels:            kernelValue,
	}, nil
}

func validateOptionalIdentifier(path, value string) error {
	if value == "" {
		return nil
	}
	return validateRequiredIdentifier(path, value, ErrInvalidSignature)
}

func validateOptionalName(path, value string) error {
	if value == "" {
		return nil
	}
	if !namePattern.MatchString(value) {
		return fmt.Errorf("%w: %s is invalid", ErrInvalidSignature, path)
	}
	return nil
}

func validateOptionalRevision(path, value string) error {
	if value != "" && !revisionPattern.MatchString(value) {
		return fmt.Errorf("%w: %s must be an immutable 40- or 64-character lowercase hexadecimal revision", ErrInvalidSignature, path)
	}
	return nil
}

func validateOptionalDigest(path, value string) error {
	if value != "" && !digestPattern.MatchString(value) {
		return fmt.Errorf("%w: %s must be a sha256 digest", ErrInvalidSignature, path)
	}
	return nil
}

func validateRequiredIdentifier(path, value string, sentinel error) error {
	if value == "" || len(value) > maxIdentifierBytes || !utf8.ValidString(value) {
		return fmt.Errorf("%w: %s must be valid UTF-8 and contain 1..%d bytes", sentinel, path, maxIdentifierBytes)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: %s contains control characters", sentinel, path)
		}
	}
	return nil
}

func isMutableAlias(value string) bool {
	if value == "" {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(value))
	if _, exists := mutableAliases[normalized]; exists {
		return true
	}
	return strings.ContainsAny(normalized, "*^~<>=") || strings.HasSuffix(normalized, ".x") ||
		strings.HasSuffix(normalized, "-snapshot") || strings.HasSuffix(normalized, ".snapshot")
}
