package safetycase

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/gaurav-gs7/InferLab/internal/strictjson"
	"github.com/gaurav-gs7/InferLab/pkg/gate"
)

const maxCaseBytes = 64 << 20

// Assemble creates a manifest from a validated descriptor and files below
// root. It re-evaluates the gate rather than trusting a supplied result.
func Assemble(root string, descriptor Descriptor) (Manifest, error) {
	if err := ValidateDescriptor(descriptor); err != nil {
		return Manifest{}, err
	}
	inputs := []ArtifactInput{
		{Name: "gate-evaluation", Role: RoleEvaluation, Path: descriptor.EvaluationPath, MediaType: "application/json"},
		{Name: "gate-result", Role: RoleResult, Path: descriptor.ResultPath, MediaType: "application/json"},
	}
	inputs = append(inputs, descriptor.Artifacts...)
	records, content, err := loadInputs(root, inputs)
	if err != nil {
		return Manifest{}, err
	}
	evaluation, result, expected, err := linkedGateDocuments(content[RoleEvaluation], content[RoleResult])
	if err != nil {
		return Manifest{}, err
	}
	if err := requireEvidenceClosure(evaluation, records); err != nil {
		return Manifest{}, err
	}
	if result.Decision == gate.DecisionBlock {
		if err := requireCounterexample(content[RoleCounterexample], evaluation, result); err != nil {
			return Manifest{}, err
		}
	}
	evaluationDigest, _ := gate.EvaluationDigest(evaluation)
	resultDigest, _ := gate.ResultDigest(result)
	manifest := Manifest{
		Schema: ManifestSchema, SchemaVersion: CurrentSchemaVersion,
		Name: descriptor.Name, CreatedAt: descriptor.CreatedAt,
		ChangeDigest: result.ChangeDigest, EvaluationDigest: evaluationDigest,
		ResultDigest: resultDigest, Decision: result.Decision,
		Artifacts: records, Claims: claimsFrom(expected), Gaps: gapsFrom(expected),
		Limitations: slices.Clone(descriptor.Limitations),
	}
	if err := ValidateManifest(manifest); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// VerifyClosure verifies every file and replays the deterministic gate. It
// performs no network access and does not invoke artifact producers.
func VerifyClosure(root string, manifest Manifest) error {
	if err := ValidateManifest(manifest); err != nil {
		return err
	}
	inputs := make([]ArtifactInput, len(manifest.Artifacts))
	for i, artifact := range manifest.Artifacts {
		inputs[i] = ArtifactInput{Name: artifact.Name, Role: artifact.Role, Path: artifact.Path, MediaType: artifact.MediaType}
	}
	records, content, err := loadInputs(root, inputs)
	if err != nil {
		return err
	}
	for i := range records {
		if records[i] != manifest.Artifacts[i] {
			return fmt.Errorf("%w: %q changed", ErrArtifactMismatch, manifest.Artifacts[i].Path)
		}
	}
	evaluation, result, expected, err := linkedGateDocuments(content[RoleEvaluation], content[RoleResult])
	if err != nil {
		return err
	}
	evaluationDigest, _ := gate.EvaluationDigest(evaluation)
	resultDigest, _ := gate.ResultDigest(result)
	if manifest.EvaluationDigest != evaluationDigest || manifest.ResultDigest != resultDigest || manifest.ChangeDigest != result.ChangeDigest || manifest.Decision != result.Decision {
		return fmt.Errorf("%w: manifest identity differs from gate documents", ErrLinkageMismatch)
	}
	if !slices.Equal(manifest.Claims, claimsFrom(expected)) || !slices.Equal(manifest.Gaps, gapsFrom(expected)) {
		return fmt.Errorf("%w: claims or gaps differ from replayed result", ErrLinkageMismatch)
	}
	if err := requireEvidenceClosure(evaluation, records); err != nil {
		return err
	}
	if result.Decision == gate.DecisionBlock {
		return requireCounterexample(content[RoleCounterexample], evaluation, result)
	}
	return nil
}

func loadInputs(root string, inputs []ArtifactInput) ([]ArtifactRecord, map[ArtifactRole][][]byte, error) {
	if len(inputs) < 2 || len(inputs) > MaxArtifacts {
		return nil, nil, fmt.Errorf("%w: artifact count is outside bounds", ErrArtifactMismatch)
	}
	records := make([]ArtifactRecord, 0, len(inputs))
	content := make(map[ArtifactRole][][]byte)
	var total int64
	for _, input := range inputs {
		data, path, err := readBoundedRegular(root, input.Path)
		if err != nil {
			return nil, nil, err
		}
		total += int64(len(data))
		if total > maxCaseBytes {
			return nil, nil, fmt.Errorf("%w: case exceeds %d bytes", ErrArtifactMismatch, maxCaseBytes)
		}
		records = append(records, ArtifactRecord{
			Name: input.Name, Role: input.Role, Path: path, MediaType: input.MediaType,
			Digest: sha256Digest(data), SizeBytes: uint64(len(data)),
		})
		content[input.Role] = append(content[input.Role], data)
	}
	return records, content, nil
}

func readBoundedRegular(root, relative string) ([]byte, string, error) {
	if filepath.IsAbs(relative) || relative == "" || strings.Contains(relative, "\\") {
		return nil, "", fmt.Errorf("%w: %q", ErrUnsafePath, relative)
	}
	clean := filepath.Clean(filepath.FromSlash(relative))
	canonical := filepath.ToSlash(clean)
	if clean == "." || clean == ".." || strings.HasPrefix(canonical, "../") || canonical != relative {
		return nil, "", fmt.Errorf("%w: %q", ErrUnsafePath, relative)
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return nil, "", fmt.Errorf("resolve case root: %w", err)
	}
	rootResolved, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, "", fmt.Errorf("%w: resolve case root: %v", ErrUnsafePath, err)
	}
	path := filepath.Join(rootAbs, clean)
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, "", fmt.Errorf("%w: read %q: %v", ErrArtifactMismatch, relative, err)
	}
	expected := filepath.Join(rootResolved, clean)
	if filepath.Clean(resolved) != filepath.Clean(expected) {
		return nil, "", fmt.Errorf("%w: %q traverses a symlink", ErrUnsafePath, relative)
	}
	within, err := filepath.Rel(rootResolved, resolved)
	if err != nil || within == ".." || strings.HasPrefix(filepath.ToSlash(within), "../") {
		return nil, "", fmt.Errorf("%w: %q", ErrUnsafePath, relative)
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return nil, "", fmt.Errorf("%w: read %q: %v", ErrArtifactMismatch, relative, err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > MaxArtifactBytes {
		return nil, "", fmt.Errorf("%w: %q is not a bounded regular file", ErrArtifactMismatch, relative)
	}
	file, err := os.Open(resolved)
	if err != nil {
		return nil, "", fmt.Errorf("%w: open %q: %v", ErrArtifactMismatch, relative, err)
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, MaxArtifactBytes+1))
	if err != nil || len(data) > MaxArtifactBytes {
		return nil, "", fmt.Errorf("%w: read %q", ErrArtifactMismatch, relative)
	}
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(info, opened) {
		return nil, "", fmt.Errorf("%w: %q changed while reading", ErrArtifactMismatch, relative)
	}
	return data, canonical, nil
}

func linkedGateDocuments(evaluationData, resultData [][]byte) (gate.Evaluation, gate.Result, gate.Result, error) {
	if len(evaluationData) != 1 || len(resultData) != 1 {
		return gate.Evaluation{}, gate.Result{}, gate.Result{}, fmt.Errorf("%w: exactly one evaluation and result are required", ErrLinkageMismatch)
	}
	evaluation, err := gate.DecodeEvaluation(bytes.NewReader(evaluationData[0]))
	if err != nil {
		return gate.Evaluation{}, gate.Result{}, gate.Result{}, fmt.Errorf("%w: evaluation: %v", ErrLinkageMismatch, err)
	}
	result, err := gate.DecodeResult(bytes.NewReader(resultData[0]))
	if err != nil {
		return gate.Evaluation{}, gate.Result{}, gate.Result{}, fmt.Errorf("%w: result: %v", ErrLinkageMismatch, err)
	}
	expected, err := gate.Evaluate(evaluation)
	if err != nil {
		return gate.Evaluation{}, gate.Result{}, gate.Result{}, fmt.Errorf("%w: replay: %v", ErrLinkageMismatch, err)
	}
	want, _ := gate.ResultDigest(expected)
	got, _ := gate.ResultDigest(result)
	if want != got {
		return gate.Evaluation{}, gate.Result{}, gate.Result{}, fmt.Errorf("%w: supplied result differs from deterministic replay", ErrLinkageMismatch)
	}
	return evaluation, result, expected, nil
}

func requireEvidenceClosure(evaluation gate.Evaluation, records []ArtifactRecord) error {
	available := make(map[string]struct{}, len(records))
	for _, record := range records {
		if record.Role == RoleEvidence {
			available[record.Digest] = struct{}{}
		}
	}
	for _, node := range evaluation.Evidence {
		for _, artifact := range node.Envelope.Artifacts {
			if _, exists := available[artifact.Digest]; !exists {
				return fmt.Errorf("%w: evidence artifact %s (%s) is missing", ErrArtifactMismatch, artifact.Name, artifact.Digest)
			}
		}
	}
	return nil
}

func requireCounterexample(items [][]byte, evaluation gate.Evaluation, result gate.Result) error {
	if len(items) == 0 {
		return fmt.Errorf("%w: BLOCK cases require a counterexample artifact", ErrArtifactMismatch)
	}
	counterexamples := make([]gate.Counterexample, 0, len(items))
	for _, data := range items {
		raw, err := strictjson.ReadOne(bytes.NewReader(data), MaxDocumentBytes)
		if err != nil {
			return fmt.Errorf("%w: invalid counterexample: %v", ErrArtifactMismatch, err)
		}
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		var counterexample gate.Counterexample
		if err := decoder.Decode(&counterexample); err != nil {
			return fmt.Errorf("%w: invalid counterexample: %v", ErrArtifactMismatch, err)
		}
		if err := gate.ValidateCounterexample(counterexample); err != nil {
			return fmt.Errorf("%w: invalid counterexample: %v", ErrArtifactMismatch, err)
		}
		counterexamples = append(counterexamples, counterexample)
	}
	regions := make(map[string]gate.Region, len(evaluation.Regions))
	for _, region := range evaluation.Regions {
		regions[region.Name] = region
	}
	for _, finding := range result.Findings {
		if finding.Outcome != gate.FindingBlock {
			continue
		}
		linked := false
		for _, counterexample := range counterexamples {
			if counterexampleFitsRegion(counterexample, regions[finding.Region]) {
				linked = true
				break
			}
		}
		if !linked {
			return fmt.Errorf("%w: no counterexample lies inside blocked region %q", ErrArtifactMismatch, finding.Region)
		}
	}
	return nil
}

func counterexampleFitsRegion(counterexample gate.Counterexample, region gate.Region) bool {
	if counterexample.Concurrency < region.Minimum.Concurrency || counterexample.Concurrency > region.Maximum.Concurrency || counterexample.Fault != region.Fault {
		return false
	}
	tenants := make(map[string]struct{})
	for _, request := range counterexample.Requests {
		if request.PromptTokens < region.Minimum.PromptTokens || request.PromptTokens > region.Maximum.PromptTokens || request.OutputTokens < region.Minimum.OutputTokens || request.OutputTokens > region.Maximum.OutputTokens {
			return false
		}
		tenants[request.Tenant] = struct{}{}
	}
	count := uint32(len(tenants))
	return count >= region.Minimum.TenantCount && count <= region.Maximum.TenantCount
}

func claimsFrom(result gate.Result) []Claim {
	claims := make([]Claim, 0, len(result.Findings))
	for _, finding := range result.Findings {
		claims = append(claims, Claim{RuleID: finding.RuleID, Outcome: finding.Outcome, Code: finding.Code, EvidenceDigest: finding.EvidenceDigest, Message: finding.Message})
	}
	slices.SortFunc(claims, func(a, b Claim) int {
		return strings.Compare(a.RuleID+"\x00"+string(a.Code)+"\x00"+a.EvidenceDigest, b.RuleID+"\x00"+string(b.Code)+"\x00"+b.EvidenceDigest)
	})
	return claims
}

func gapsFrom(result gate.Result) []string {
	gaps := make([]string, 0)
	for _, finding := range result.Findings {
		if finding.Outcome == gate.FindingInconclusive {
			gaps = append(gaps, finding.RuleID+": "+string(finding.Code)+": "+finding.Message)
		}
	}
	return sortedUnique(gaps)
}

func sha256Digest(data []byte) string {
	digest := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(digest[:])
}
