package safetycase

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gaurav-gs7/InferLab/pkg/gate"
)

func validDescriptor() Descriptor {
	return Descriptor{
		Schema: DescriptorSchema, SchemaVersion: CurrentSchemaVersion,
		Name: "validation-case", CreatedAt: "2026-07-16T08:00:00Z",
		EvaluationPath: "evaluation.json", ResultPath: "result.json",
		Artifacts: []ArtifactInput{
			{Name: "evidence", Role: RoleEvidence, Path: "evidence.json", MediaType: "application/json"},
			{Name: "supporting", Role: RoleSupporting, Path: "supporting.txt", MediaType: "text/plain"},
		},
		Limitations: []string{"synthetic fixture"},
	}
}

func TestValidateDescriptorBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*Descriptor)
	}{
		{name: "schema", mutate: func(value *Descriptor) { value.Schema = "other" }},
		{name: "name", mutate: func(value *Descriptor) { value.Name = "Bad Name" }},
		{name: "created text", mutate: func(value *Descriptor) { value.CreatedAt = "line one\nline two" }},
		{name: "created time", mutate: func(value *Descriptor) { value.CreatedAt = "tomorrow" }},
		{name: "same paths", mutate: func(value *Descriptor) { value.ResultPath = value.EvaluationPath }},
		{name: "empty path", mutate: func(value *Descriptor) { value.EvaluationPath = "" }},
		{name: "unsafe path", mutate: func(value *Descriptor) { value.EvaluationPath = "../evaluation.json" }},
		{name: "too many artifacts", mutate: func(value *Descriptor) { value.Artifacts = make([]ArtifactInput, MaxArtifacts-1) }},
		{name: "artifact name", mutate: func(value *Descriptor) { value.Artifacts[0].Name = "Bad Name" }},
		{name: "reserved artifact role", mutate: func(value *Descriptor) { value.Artifacts[0].Role = RoleEvaluation }},
		{name: "unknown artifact role", mutate: func(value *Descriptor) { value.Artifacts[0].Role = ArtifactRole("other") }},
		{name: "artifact media type", mutate: func(value *Descriptor) { value.Artifacts[0].MediaType = "json" }},
		{name: "duplicate artifact name", mutate: func(value *Descriptor) { value.Artifacts[1].Name = value.Artifacts[0].Name }},
		{name: "reserved artifact path", mutate: func(value *Descriptor) { value.Artifacts[0].Path = value.EvaluationPath }},
		{name: "unsafe artifact path", mutate: func(value *Descriptor) { value.Artifacts[0].Path = "nested/../evidence.json" }},
		{name: "too many limitations", mutate: func(value *Descriptor) { value.Limitations = make([]string, 129) }},
		{name: "invalid limitation", mutate: func(value *Descriptor) { value.Limitations = []string{"line one\nline two"} }},
		{name: "duplicate limitation", mutate: func(value *Descriptor) { value.Limitations = []string{"same", "same"} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			descriptor := validDescriptor()
			tt.mutate(&descriptor)
			if err := ValidateDescriptor(descriptor); !errors.Is(err, ErrInvalidDescriptor) {
				t.Fatalf("ValidateDescriptor() error = %v, want %v", err, ErrInvalidDescriptor)
			}
		})
	}

	encoded, err := json.Marshal(validDescriptor())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodeDescriptor(bytes.NewReader(encoded)); err != nil {
		t.Fatalf("DecodeDescriptor() error = %v", err)
	}
	if _, err := DecodeDescriptor(bytes.NewReader([]byte(`{"unknown":true}`))); !errors.Is(err, ErrInvalidDescriptor) {
		t.Fatalf("DecodeDescriptor() error = %v, want %v", err, ErrInvalidDescriptor)
	}
}

func TestManifestAdditionalBoundaries(t *testing.T) {
	t.Parallel()

	base := Manifest{
		Schema: ManifestSchema, SchemaVersion: CurrentSchemaVersion,
		Name: "validation-case", CreatedAt: "2026-07-16T08:00:00Z",
		ChangeDigest: digestOf('a'), EvaluationDigest: digestOf('b'), ResultDigest: digestOf('c'), Decision: gate.DecisionInconclusive,
		Artifacts: []ArtifactRecord{
			{Name: "gate-evaluation", Role: RoleEvaluation, Path: "evaluation.json", MediaType: "application/json", Digest: digestOf('d'), SizeBytes: 1},
			{Name: "gate-result", Role: RoleResult, Path: "result.json", MediaType: "application/json", Digest: digestOf('e'), SizeBytes: 1},
		},
		Claims: []Claim{{RuleID: "rule", Outcome: gate.FindingInconclusive, Code: gate.CodeMissingCoverage, Message: "missing"}},
		Gaps:   []string{}, Limitations: []string{},
	}
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{name: "artifact media", mutate: func(value *Manifest) { value.Artifacts[0].MediaType = "json" }},
		{name: "artifact digest", mutate: func(value *Manifest) { value.Artifacts[0].Digest = "bad" }},
		{name: "artifact path", mutate: func(value *Manifest) { value.Artifacts[0].Path = "../evaluation.json" }},
		{name: "artifact size", mutate: func(value *Manifest) { value.Artifacts[0].SizeBytes = MaxArtifactBytes + 1 }},
		{name: "duplicate artifact path", mutate: func(value *Manifest) { value.Artifacts[1].Path = value.Artifacts[0].Path }},
		{name: "claim identity", mutate: func(value *Manifest) { value.Claims[0].RuleID = "Bad ID" }},
		{name: "claim message", mutate: func(value *Manifest) { value.Claims[0].Message = "line one\nline two" }},
		{name: "claim evidence digest", mutate: func(value *Manifest) { value.Claims[0].EvidenceDigest = "bad" }},
		{name: "claim outcome", mutate: func(value *Manifest) { value.Claims[0].Outcome = gate.FindingOutcome("unknown") }},
		{name: "limitations bound", mutate: func(value *Manifest) { value.Limitations = make([]string, 129) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidate := base
			candidate.Artifacts = append([]ArtifactRecord(nil), base.Artifacts...)
			candidate.Claims = append([]Claim(nil), base.Claims...)
			tt.mutate(&candidate)
			if err := ValidateManifest(candidate); !errors.Is(err, ErrInvalidManifest) {
				t.Fatalf("ValidateManifest() error = %v, want %v", err, ErrInvalidManifest)
			}
		})
	}
}

func TestLinkedGateDocumentsBoundaries(t *testing.T) {
	t.Parallel()

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
	if _, _, _, err := linkedGateDocuments([][]byte{evaluationData}, [][]byte{resultData}); err != nil {
		t.Fatalf("linkedGateDocuments() error = %v", err)
	}
	tests := []struct {
		name        string
		evaluations [][]byte
		results     [][]byte
	}{
		{name: "missing evaluation", results: [][]byte{resultData}},
		{name: "invalid evaluation", evaluations: [][]byte{[]byte(`{}`)}, results: [][]byte{resultData}},
		{name: "invalid result", evaluations: [][]byte{evaluationData}, results: [][]byte{[]byte(`{}`)}},
	}
	for _, tt := range tests {
		if _, _, _, err := linkedGateDocuments(tt.evaluations, tt.results); !errors.Is(err, ErrLinkageMismatch) {
			t.Fatalf("%s error = %v, want %v", tt.name, err, ErrLinkageMismatch)
		}
	}

	changed := evaluation
	changed.Name = "changed-evaluation"
	changedData, err := gate.CanonicalEvaluationJSON(changed)
	if err != nil {
		t.Fatal(err)
	}
	changedResult, err := gate.Evaluate(changed)
	if err != nil {
		t.Fatal(err)
	}
	changedResultData, err := gate.CanonicalResultJSON(changedResult)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := linkedGateDocuments([][]byte{evaluationData}, [][]byte{changedResultData}); !errors.Is(err, ErrLinkageMismatch) {
		t.Fatalf("replay mismatch error = %v, want %v; changed=%s", err, ErrLinkageMismatch, changedData)
	}
}

func TestPathAndCounterexampleBoundaries(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "regular.json", []byte(`{}`))
	if _, _, err := loadInputs(root, []ArtifactInput{{Path: "regular.json"}}); !errors.Is(err, ErrArtifactMismatch) {
		t.Fatalf("loadInputs() error = %v, want %v", err, ErrArtifactMismatch)
	}
	for _, path := range []string{"", "/absolute", `back\\slash`, ".", "nested/../regular.json"} {
		if _, _, err := readBoundedRegular(root, path); !errors.Is(err, ErrUnsafePath) {
			t.Fatalf("readBoundedRegular(%q) error = %v, want %v", path, err, ErrUnsafePath)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "directory"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, _, err := readBoundedRegular(root, "directory"); !errors.Is(err, ErrArtifactMismatch) {
		t.Fatalf("directory error = %v, want %v", err, ErrArtifactMismatch)
	}

	evaluationData, err := os.ReadFile(filepath.Join("..", "..", "examples", "block-gate.json"))
	if err != nil {
		t.Fatal(err)
	}
	evaluation, err := gate.DecodeEvaluation(bytes.NewReader(evaluationData))
	if err != nil {
		t.Fatal(err)
	}
	counterexampleData, err := os.ReadFile(filepath.Join("..", "..", "examples", "block-counterexample.json"))
	if err != nil {
		t.Fatal(err)
	}
	var counterexample gate.Counterexample
	if err := json.Unmarshal(counterexampleData, &counterexample); err != nil {
		t.Fatal(err)
	}
	region := evaluation.Regions[0]
	if !counterexampleFitsRegion(counterexample, region) {
		t.Fatal("fixture counterexample did not fit its blocked region")
	}
	outside := counterexample
	outside.Concurrency = region.Maximum.Concurrency + 1
	if counterexampleFitsRegion(outside, region) {
		t.Fatal("counterexampleFitsRegion() accepted excessive concurrency")
	}
	outside = counterexample
	outside.Requests = append([]gate.CounterexampleRequest(nil), counterexample.Requests...)
	outside.Requests[0].PromptTokens = region.Maximum.PromptTokens + 1
	if counterexampleFitsRegion(outside, region) {
		t.Fatal("counterexampleFitsRegion() accepted an out-of-range request")
	}
	outside = counterexample
	outside.Requests = append([]gate.CounterexampleRequest(nil), counterexample.Requests...)
	for i := range outside.Requests {
		outside.Requests[i].Tenant = "one-tenant"
	}
	if counterexampleFitsRegion(outside, region) {
		t.Fatal("counterexampleFitsRegion() accepted too few tenants")
	}
	if err := requireCounterexample([][]byte{[]byte(`not-json`)}, evaluation, gate.Result{Decision: gate.DecisionBlock}); !errors.Is(err, ErrArtifactMismatch) {
		t.Fatalf("invalid counterexample error = %v, want %v", err, ErrArtifactMismatch)
	}
}

func TestSignatureAdditionalBoundaries(t *testing.T) {
	t.Parallel()

	if _, err := KeyID(ed25519.PublicKey("short")); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("KeyID() error = %v, want %v", err, ErrInvalidSignature)
	}
	for _, value := range [][]byte{nil, []byte("not pem"), []byte("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\ntrailing")} {
		if _, err := ParsePrivateKeyPEM(value); !errors.Is(err, ErrInvalidSignature) {
			t.Fatalf("ParsePrivateKeyPEM() error = %v, want %v", err, ErrInvalidSignature)
		}
	}
	for _, value := range [][]byte{nil, []byte("not pem"), []byte("-----BEGIN PUBLIC KEY-----\nAAAA\n-----END PUBLIC KEY-----\ntrailing")} {
		if _, err := ParsePublicKeyPEM(value); !errors.Is(err, ErrInvalidSignature) {
			t.Fatalf("ParsePublicKeyPEM() error = %v, want %v", err, ErrInvalidSignature)
		}
	}
	wrongAlgorithm, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(wrongAlgorithm)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePrivateKeyPEM(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER})); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("non-Ed25519 private key error = %v, want %v", err, ErrInvalidSignature)
	}
	publicDER, err := x509.MarshalPKIXPublicKey(&wrongAlgorithm.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ParsePublicKeyPEM(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicDER})); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("non-Ed25519 public key error = %v, want %v", err, ErrInvalidSignature)
	}
	if _, err := DecodeSignature(bytes.NewReader([]byte(`{"schema":"inferlab.safety-case-signature"}`))); !errors.Is(err, ErrInvalidSignature) {
		t.Fatalf("DecodeSignature() error = %v, want %v", err, ErrInvalidSignature)
	}
	if !validText("safe", 4) || validText("", 4) || validText(strings.Repeat("x", 5), 4) || validText("line\n", 8) {
		t.Fatal("validText() contract mismatch")
	}
	if !validRelativePath("nested/artifact.json") {
		t.Fatal("validRelativePath() rejected a safe path")
	}
	for _, path := range []string{"", "/absolute", `back\slash`, ".", "..", "../outside", "nested/../artifact.json"} {
		if validRelativePath(path) {
			t.Fatalf("validRelativePath(%q) accepted an unsafe path", path)
		}
	}
}
