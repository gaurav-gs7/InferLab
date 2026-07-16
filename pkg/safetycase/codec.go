package safetycase

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/gaurav-gs7/InferLab/internal/strictjson"
	"github.com/gaurav-gs7/InferLab/pkg/gate"
)

var (
	namePattern      = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9._-]{0,126}[a-z0-9])?$`)
	digestPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	mediaTypePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9!#$&^_.+-]*/[a-z0-9][a-z0-9!#$&^_.+-]*$`)
)

func DecodeDescriptor(reader io.Reader) (Descriptor, error) {
	value, err := decodeStrict[Descriptor](reader, ErrInvalidDescriptor)
	if err != nil {
		return Descriptor{}, err
	}
	if err := ValidateDescriptor(value); err != nil {
		return Descriptor{}, err
	}
	return value, nil
}

func DecodeManifest(reader io.Reader) (Manifest, error) {
	value, err := decodeStrict[Manifest](reader, ErrInvalidManifest)
	if err != nil {
		return Manifest{}, err
	}
	if err := ValidateManifest(value); err != nil {
		return Manifest{}, err
	}
	return value, nil
}

func DecodeSignature(reader io.Reader) (Signature, error) {
	value, err := decodeStrict[Signature](reader, ErrInvalidSignature)
	if err != nil {
		return Signature{}, err
	}
	if err := validateSignature(value); err != nil {
		return Signature{}, err
	}
	return value, nil
}

func decodeStrict[T any](reader io.Reader, sentinel error) (T, error) {
	var zero T
	data, err := strictjson.ReadOne(reader, MaxDocumentBytes)
	if err != nil {
		return zero, fmt.Errorf("%w: %v", sentinel, err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value T
	if err := decoder.Decode(&value); err != nil {
		return zero, fmt.Errorf("%w: %v", sentinel, err)
	}
	return value, nil
}

func ValidateDescriptor(value Descriptor) error {
	if value.Schema != DescriptorSchema || value.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("%w: unsupported schema or version", ErrInvalidDescriptor)
	}
	if !namePattern.MatchString(value.Name) || !validText(value.CreatedAt, 128) {
		return fmt.Errorf("%w: invalid name or created_at", ErrInvalidDescriptor)
	}
	if _, err := time.Parse(time.RFC3339Nano, value.CreatedAt); err != nil {
		return fmt.Errorf("%w: created_at must be RFC3339", ErrInvalidDescriptor)
	}
	if value.EvaluationPath == value.ResultPath || value.EvaluationPath == "" || value.ResultPath == "" {
		return fmt.Errorf("%w: distinct evaluation_path and result_path are required", ErrInvalidDescriptor)
	}
	if !validRelativePath(value.EvaluationPath) || !validRelativePath(value.ResultPath) {
		return fmt.Errorf("%w: evaluation_path or result_path is unsafe", ErrInvalidDescriptor)
	}
	if len(value.Artifacts) > MaxArtifacts-2 {
		return fmt.Errorf("%w: too many artifacts", ErrInvalidDescriptor)
	}
	names := map[string]struct{}{"gate-evaluation": {}, "gate-result": {}}
	paths := map[string]struct{}{value.EvaluationPath: {}, value.ResultPath: {}}
	for i, artifact := range value.Artifacts {
		if !namePattern.MatchString(artifact.Name) || !validRole(artifact.Role) || artifact.Role == RoleEvaluation || artifact.Role == RoleResult || !mediaTypePattern.MatchString(artifact.MediaType) {
			return fmt.Errorf("%w: artifacts[%d] is invalid", ErrInvalidDescriptor, i)
		}
		if _, exists := names[artifact.Name]; exists {
			return fmt.Errorf("%w: duplicate artifact name %q", ErrInvalidDescriptor, artifact.Name)
		}
		if _, exists := paths[artifact.Path]; exists || !validRelativePath(artifact.Path) {
			return fmt.Errorf("%w: duplicate or empty artifact path", ErrInvalidDescriptor)
		}
		names[artifact.Name], paths[artifact.Path] = struct{}{}, struct{}{}
	}
	return validateTextSet(value.Limitations, 128, "limitations", ErrInvalidDescriptor)
}

func ValidateManifest(value Manifest) error {
	if value.Schema != ManifestSchema || value.SchemaVersion != CurrentSchemaVersion {
		return fmt.Errorf("%w: unsupported schema or version", ErrInvalidManifest)
	}
	if !namePattern.MatchString(value.Name) || !digestPattern.MatchString(value.ChangeDigest) || !digestPattern.MatchString(value.EvaluationDigest) || !digestPattern.MatchString(value.ResultDigest) {
		return fmt.Errorf("%w: invalid identity or digest", ErrInvalidManifest)
	}
	if _, err := time.Parse(time.RFC3339Nano, value.CreatedAt); err != nil {
		return fmt.Errorf("%w: created_at must be RFC3339", ErrInvalidManifest)
	}
	if value.Decision != gate.DecisionPass && value.Decision != gate.DecisionBlock && value.Decision != gate.DecisionInconclusive {
		return fmt.Errorf("%w: invalid decision", ErrInvalidManifest)
	}
	if len(value.Artifacts) < 2 || len(value.Artifacts) > MaxArtifacts || len(value.Claims) == 0 {
		return fmt.Errorf("%w: artifacts or claims are outside bounds", ErrInvalidManifest)
	}
	names, paths := map[string]struct{}{}, map[string]struct{}{}
	roles := map[ArtifactRole]int{}
	for i, artifact := range value.Artifacts {
		if !namePattern.MatchString(artifact.Name) || !validRole(artifact.Role) || !mediaTypePattern.MatchString(artifact.MediaType) || !digestPattern.MatchString(artifact.Digest) || !validRelativePath(artifact.Path) || artifact.SizeBytes > MaxArtifactBytes {
			return fmt.Errorf("%w: artifacts[%d] is invalid", ErrInvalidManifest, i)
		}
		if _, exists := names[artifact.Name]; exists {
			return fmt.Errorf("%w: duplicate artifact name", ErrInvalidManifest)
		}
		if _, exists := paths[artifact.Path]; exists {
			return fmt.Errorf("%w: duplicate artifact path", ErrInvalidManifest)
		}
		names[artifact.Name], paths[artifact.Path] = struct{}{}, struct{}{}
		roles[artifact.Role]++
	}
	if roles[RoleEvaluation] != 1 || roles[RoleResult] != 1 {
		return fmt.Errorf("%w: exactly one evaluation and result artifact are required", ErrInvalidManifest)
	}
	for i, claim := range value.Claims {
		if !namePattern.MatchString(claim.RuleID) || !validText(claim.Message, 1024) || claim.EvidenceDigest != "" && !digestPattern.MatchString(claim.EvidenceDigest) || !validClaimCode(claim.Code) {
			return fmt.Errorf("%w: claims[%d] is invalid", ErrInvalidManifest, i)
		}
		if claim.Outcome != gate.FindingPass && claim.Outcome != gate.FindingBlock && claim.Outcome != gate.FindingInconclusive {
			return fmt.Errorf("%w: claims[%d] outcome is invalid", ErrInvalidManifest, i)
		}
		if claim.Outcome == gate.FindingPass && claim.Code != gate.CodeWithinPolicy || claim.Outcome == gate.FindingBlock && claim.Code != gate.CodeViolationRisk || claim.Outcome == gate.FindingInconclusive && (claim.Code == gate.CodeWithinPolicy || claim.Code == gate.CodeViolationRisk) {
			return fmt.Errorf("%w: claims[%d] outcome and code disagree", ErrInvalidManifest, i)
		}
	}
	if err := validateTextSet(value.Gaps, 256, "gaps", ErrInvalidManifest); err != nil {
		return err
	}
	return validateTextSet(value.Limitations, 128, "limitations", ErrInvalidManifest)
}

func CanonicalManifestJSON(value Manifest) ([]byte, error) {
	if err := ValidateManifest(value); err != nil {
		return nil, err
	}
	canonical := value
	canonical.Artifacts = slices.Clone(value.Artifacts)
	slices.SortFunc(canonical.Artifacts, func(a, b ArtifactRecord) int { return strings.Compare(a.Name, b.Name) })
	canonical.Claims = slices.Clone(value.Claims)
	slices.SortFunc(canonical.Claims, func(a, b Claim) int {
		return strings.Compare(a.RuleID+"\x00"+string(a.Code)+"\x00"+a.EvidenceDigest, b.RuleID+"\x00"+string(b.Code)+"\x00"+b.EvidenceDigest)
	})
	canonical.Gaps = sortedUnique(value.Gaps)
	canonical.Limitations = sortedUnique(value.Limitations)
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return nil, fmt.Errorf("encode canonical safety case: %w", err)
	}
	return encoded, nil
}

func ManifestDigest(value Manifest) (string, error) {
	encoded, err := CanonicalManifestJSON(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func CanonicalSignatureJSON(value Signature) ([]byte, error) {
	if err := validateSignature(value); err != nil {
		return nil, err
	}
	return json.Marshal(value)
}

func validRole(role ArtifactRole) bool {
	return role == RoleEvaluation || role == RoleResult || role == RoleEvidence || role == RoleCounterexample || role == RoleSupporting
}

func validText(value string, limit int) bool {
	if value == "" || len(value) > limit || !utf8.ValidString(value) {
		return false
	}
	for _, character := range value {
		if unicode.IsControl(character) {
			return false
		}
	}
	return true
}

func validateTextSet(values []string, limit int, field string, sentinel error) error {
	if len(values) > limit {
		return fmt.Errorf("%w: %s exceeds bounds", sentinel, field)
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !validText(value, 1024) {
			return fmt.Errorf("%w: %s contains invalid text", sentinel, field)
		}
		if _, exists := seen[value]; exists {
			return fmt.Errorf("%w: %s contains duplicates", sentinel, field)
		}
		seen[value] = struct{}{}
	}
	return nil
}

func sortedUnique(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	result := slices.Clone(values)
	slices.Sort(result)
	return slices.Compact(result)
}

func validRelativePath(value string) bool {
	if value == "" || filepath.IsAbs(value) || strings.Contains(value, "\\") {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(filepath.FromSlash(value)))
	return clean == value && clean != "." && clean != ".." && !strings.HasPrefix(clean, "../")
}

func validClaimCode(code gate.FindingCode) bool {
	switch code {
	case gate.CodeWithinPolicy, gate.CodeViolationRisk, gate.CodeUncertaintyOverlap, gate.CodeMissingCoverage,
		gate.CodeOutOfDistribution, gate.CodeMissingMetric, gate.CodeInvalidMetric, gate.CodeMissingUncertainty,
		gate.CodeUnderSampled, gate.CodeIncompleteEvidence, gate.CodeNonObservedEvidence, gate.CodeStaleEvidence,
		gate.CodeFutureEvidence, gate.CodeRuntimeIncompatible, gate.CodeRuntimeUnknown:
		return true
	default:
		return false
	}
}

func validateSignature(value Signature) error {
	if value.Schema != SignatureSchema || value.SchemaVersion != CurrentSchemaVersion || value.Algorithm != "ed25519" || !digestPattern.MatchString(value.ManifestDigest) || !digestPattern.MatchString(value.KeyID) || value.Value == "" {
		return fmt.Errorf("%w: invalid fields", ErrInvalidSignature)
	}
	decoded, err := base64.RawStdEncoding.DecodeString(value.Value)
	if err != nil || len(decoded) != 64 {
		return fmt.Errorf("%w: value is not one Ed25519 signature", ErrInvalidSignature)
	}
	return nil
}
