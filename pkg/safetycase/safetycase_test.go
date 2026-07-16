package safetycase

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gaurav-gs7/InferLab/pkg/gate"
)

func TestSignedOfflineClosure(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	evaluationData, err := os.ReadFile(filepath.Join("..", "..", "examples", "missing-evidence-gate.json"))
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := gate.DecodeEvaluation(bytes.NewReader(evaluationData))
	if err != nil {
		t.Fatal(err)
	}
	result, err := gate.Evaluate(evaluation)
	if err != nil {
		t.Fatal(err)
	}
	resultData, err := gate.CanonicalResultJSON(result)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "evaluation.json", evaluationData)
	writeFile(t, root, "result.json", resultData)
	descriptor := Descriptor{
		Schema: DescriptorSchema, SchemaVersion: CurrentSchemaVersion,
		Name: "offline-case", CreatedAt: "2026-07-16T08:00:00Z",
		EvaluationPath: "evaluation.json", ResultPath: "result.json",
		Limitations: []string{"Synthetic public fixture; no production safety claim."},
	}
	manifest, err := Assemble(root, descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Decision != gate.DecisionInconclusive || len(manifest.Gaps) != 1 {
		t.Fatalf("unexpected manifest: %#v", manifest)
	}
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	signature, err := Sign(manifest, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature(manifest, signature, publicKey); err != nil {
		t.Fatal(err)
	}
	if err := VerifyClosure(root, manifest); err != nil {
		t.Fatal(err)
	}
	encodedSignature, err := CanonicalSignatureJSON(signature)
	if err != nil {
		t.Fatal(err)
	}
	decodedSignature, err := DecodeSignature(bytes.NewReader(encodedSignature))
	if err != nil || decodedSignature != signature {
		t.Fatalf("signature round trip = %#v, %v", decodedSignature, err)
	}
	wrongPublic, _, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifySignature(manifest, signature, wrongPublic); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("wrong-key error = %v", err)
	}

	tamperedSignature := signature
	tamperedSignature.Value = "AAAA"
	if err := VerifySignature(manifest, tamperedSignature, publicKey); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("tampered signature error = %v", err)
	}
	writeFile(t, root, "result.json", append(resultData, ' '))
	if err := VerifyClosure(root, manifest); !errors.Is(err, ErrArtifactMismatch) {
		t.Fatalf("tampered artifact error = %v", err)
	}
	writeFile(t, root, "result.json", resultData)
	tamperedManifest := manifest
	tamperedManifest.Claims = append([]Claim(nil), manifest.Claims...)
	tamperedManifest.Claims[0].Message = "rewritten claim"
	if err := VerifyClosure(root, tamperedManifest); !errors.Is(err, ErrLinkageMismatch) {
		t.Fatalf("tampered claim error = %v", err)
	}
	if err := os.Remove(filepath.Join(root, "evaluation.json")); err != nil {
		t.Fatal(err)
	}
	if err := VerifyClosure(root, manifest); !errors.Is(err, ErrArtifactMismatch) {
		t.Fatalf("missing artifact error = %v", err)
	}
}

func TestBlockClosureBindsEvidenceAndCounterexampleRegion(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	examples := filepath.Join("..", "..", "examples")
	evaluationData, err := os.ReadFile(filepath.Join(examples, "block-gate.json"))
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := gate.DecodeEvaluation(bytes.NewReader(evaluationData))
	if err != nil {
		t.Fatal(err)
	}
	result, err := gate.Evaluate(evaluation)
	if err != nil {
		t.Fatal(err)
	}
	resultData, _ := gate.CanonicalResultJSON(result)
	writeFile(t, root, "evaluation.json", evaluationData)
	writeFile(t, root, "result.json", resultData)
	raw, err := os.ReadFile(filepath.Join(examples, "artifacts", "block-observation.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "raw.json", raw)
	counterexample, err := os.ReadFile(filepath.Join(examples, "block-counterexample.json"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "counterexample.json", counterexample)
	descriptor := Descriptor{
		Schema: DescriptorSchema, SchemaVersion: CurrentSchemaVersion,
		Name: "block-case", CreatedAt: "2026-07-16T10:00:00Z",
		EvaluationPath: "evaluation.json", ResultPath: "result.json",
		Artifacts: []ArtifactInput{
			{Name: "raw", Role: RoleEvidence, Path: "raw.json", MediaType: "application/json"},
			{Name: "counterexample", Role: RoleCounterexample, Path: "counterexample.json", MediaType: "application/json"},
		},
	}
	manifest, err := Assemble(root, descriptor)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.Decision != gate.DecisionBlock {
		t.Fatalf("decision = %s, want BLOCK", manifest.Decision)
	}

	bad := bytes.ReplaceAll(counterexample, []byte(`"tenant-2"`), []byte(`"tenant-1"`))
	writeFile(t, root, "counterexample.json", bad)
	if _, err := Assemble(root, descriptor); !errors.Is(err, ErrArtifactMismatch) {
		t.Fatalf("out-of-region counterexample error = %v", err)
	}
	writeFile(t, root, "counterexample.json", counterexample)
	descriptor.Artifacts = descriptor.Artifacts[1:]
	if _, err := Assemble(root, descriptor); !errors.Is(err, ErrArtifactMismatch) {
		t.Fatalf("missing evidence error = %v", err)
	}
}

func TestClosureRejectsUnsafeOrMissingArtifacts(t *testing.T) {
	t.Parallel()
	descriptor := Descriptor{
		Schema: DescriptorSchema, SchemaVersion: CurrentSchemaVersion,
		Name: "unsafe-case", CreatedAt: "2026-07-16T08:00:00Z",
		EvaluationPath: "../evaluation.json", ResultPath: "result.json",
	}
	_, err := Assemble(t.TempDir(), descriptor)
	if !errors.Is(err, ErrInvalidDescriptor) {
		t.Fatalf("Assemble() error = %v, want ErrInvalidDescriptor", err)
	}
	if _, _, err := readBoundedRegular(t.TempDir(), "../evaluation.json"); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("readBoundedRegular() error = %v, want ErrUnsafePath", err)
	}

	root := t.TempDir()
	writeFile(t, root, "evaluation.json", []byte(`{}`))
	if err := os.Symlink(filepath.Join(root, "evaluation.json"), filepath.Join(root, "result.json")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	descriptor.EvaluationPath = "evaluation.json"
	_, err = Assemble(root, descriptor)
	if !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("symlink error = %v, want ErrUnsafePath", err)
	}

	root = t.TempDir()
	outside := t.TempDir()
	writeFile(t, outside, "artifact.json", []byte(`{}`))
	if err := os.Symlink(outside, filepath.Join(root, "linked-directory")); err != nil {
		t.Skipf("directory symlinks unavailable: %v", err)
	}
	if _, _, err := readBoundedRegular(root, "linked-directory/artifact.json"); !errors.Is(err, ErrUnsafePath) {
		t.Fatalf("intermediate-symlink error = %v, want ErrUnsafePath", err)
	}
}

func TestCanonicalManifestAndStrictDecoders(t *testing.T) {
	t.Parallel()
	manifest := Manifest{
		Schema: ManifestSchema, SchemaVersion: CurrentSchemaVersion,
		Name: "canonical-case", CreatedAt: "2026-07-16T08:00:00Z",
		ChangeDigest: digestOf('a'), EvaluationDigest: digestOf('b'), ResultDigest: digestOf('c'),
		Decision: gate.DecisionInconclusive,
		Artifacts: []ArtifactRecord{
			{Name: "gate-result", Role: RoleResult, Path: "result.json", MediaType: "application/json", Digest: digestOf('d'), SizeBytes: 1},
			{Name: "gate-evaluation", Role: RoleEvaluation, Path: "evaluation.json", MediaType: "application/json", Digest: digestOf('e'), SizeBytes: 1},
		},
		Claims: []Claim{{RuleID: "ttft", Outcome: gate.FindingInconclusive, Code: gate.CodeMissingCoverage, Message: "missing"}},
		Gaps:   []string{"z", "a"}, Limitations: []string{"z", "a"},
	}
	first, err := CanonicalManifestJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Artifacts[0], manifest.Artifacts[1] = manifest.Artifacts[1], manifest.Artifacts[0]
	manifest.Gaps[0], manifest.Gaps[1] = manifest.Gaps[1], manifest.Gaps[0]
	second, err := CanonicalManifestJSON(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("canonical form depends on unordered collection order")
	}
	if _, err := DecodeManifest(bytes.NewBufferString(`{"schema":"inferlab.safety-case","schema":"duplicate"}`)); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("duplicate-field error = %v", err)
	}
	encoded, _ := json.Marshal(manifest)
	if _, err := DecodeManifest(bytes.NewReader(append(encoded, []byte("\ntrue")...))); !errors.Is(err, ErrInvalidManifest) {
		t.Fatalf("trailing-value error = %v", err)
	}
}

func TestPEMRoundTripAndWrongKey(t *testing.T) {
	t.Parallel()
	publicKey, privateKey, err := GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	privatePEM, _ := MarshalPrivateKeyPEM(privateKey)
	publicPEM, _ := MarshalPublicKeyPEM(publicKey)
	parsedPrivate, err := ParsePrivateKeyPEM(privatePEM)
	if err != nil {
		t.Fatal(err)
	}
	parsedPublic, err := ParsePublicKeyPEM(publicPEM)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(privateKey, parsedPrivate) || !bytes.Equal(publicKey, parsedPublic) {
		t.Fatal("PEM key round trip changed key material")
	}
	if _, err := ParsePrivateKeyPEM(publicPEM); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("public-as-private error = %v", err)
	}
	if _, err := ParsePublicKeyPEM(privatePEM); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("private-as-public error = %v", err)
	}
	if _, err := CanonicalSignatureJSON(Signature{}); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("empty signature error = %v", err)
	}
	if _, err := Sign(Manifest{}, []byte("short")); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("short private-key error = %v", err)
	}
	if err := VerifySignature(Manifest{}, Signature{}, []byte("short")); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("short public-key error = %v", err)
	}
	if _, err := MarshalPrivateKeyPEM([]byte("short")); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("marshal short private-key error = %v", err)
	}
	if _, err := MarshalPublicKeyPEM([]byte("short")); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("marshal short public-key error = %v", err)
	}
}

func TestManifestValidationFailsClosed(t *testing.T) {
	t.Parallel()
	valid := Manifest{
		Schema: ManifestSchema, SchemaVersion: CurrentSchemaVersion,
		Name: "validation-case", CreatedAt: "2026-07-16T08:00:00Z",
		ChangeDigest: digestOf('a'), EvaluationDigest: digestOf('b'), ResultDigest: digestOf('c'),
		Decision: gate.DecisionInconclusive,
		Artifacts: []ArtifactRecord{
			{Name: "gate-evaluation", Role: RoleEvaluation, Path: "evaluation.json", MediaType: "application/json", Digest: digestOf('d'), SizeBytes: 1},
			{Name: "gate-result", Role: RoleResult, Path: "result.json", MediaType: "application/json", Digest: digestOf('e'), SizeBytes: 1},
		},
		Claims: []Claim{{RuleID: "ttft", Outcome: gate.FindingInconclusive, Code: gate.CodeMissingCoverage, Message: "missing"}},
		Gaps:   []string{}, Limitations: []string{},
	}
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{name: "schema", mutate: func(value *Manifest) { value.Schema = "wrong" }},
		{name: "digest", mutate: func(value *Manifest) { value.ResultDigest = "wrong" }},
		{name: "time", mutate: func(value *Manifest) { value.CreatedAt = "tomorrow" }},
		{name: "decision", mutate: func(value *Manifest) { value.Decision = "MAYBE" }},
		{name: "claims absent", mutate: func(value *Manifest) { value.Claims = nil }},
		{name: "duplicate artifact", mutate: func(value *Manifest) { value.Artifacts[1].Name = value.Artifacts[0].Name }},
		{name: "missing result role", mutate: func(value *Manifest) { value.Artifacts[1].Role = RoleSupporting }},
		{name: "bad claim code", mutate: func(value *Manifest) { value.Claims[0].Code = "invented" }},
		{name: "contradictory claim", mutate: func(value *Manifest) { value.Claims[0].Outcome = gate.FindingPass }},
		{name: "duplicate gap", mutate: func(value *Manifest) { value.Gaps = []string{"same", "same"} }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			candidate := valid
			candidate.Artifacts = append([]ArtifactRecord(nil), valid.Artifacts...)
			candidate.Claims = append([]Claim(nil), valid.Claims...)
			test.mutate(&candidate)
			if err := ValidateManifest(candidate); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("ValidateManifest() error = %v", err)
			}
		})
	}
}

func writeFile(t *testing.T, root, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, name), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func digestOf(character byte) string { return "sha256:" + string(bytes.Repeat([]byte{character}, 64)) }
