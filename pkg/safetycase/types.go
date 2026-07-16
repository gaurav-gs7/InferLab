// Package safetycase creates content-addressed, signed, offline-verifiable
// release-assurance bundles without changing the meaning of gate evidence.
package safetycase

import "github.com/gaurav-gs7/InferLab/pkg/gate"

const (
	DescriptorSchema     = "inferlab.safety-case-descriptor"
	ManifestSchema       = "inferlab.safety-case"
	SignatureSchema      = "inferlab.safety-case-signature"
	CurrentSchemaVersion = "1.0"
	MaxDocumentBytes     = 8 << 20
	MaxArtifactBytes     = 16 << 20
	MaxArtifacts         = 512
)

type ArtifactRole string

const (
	RoleEvaluation     ArtifactRole = "gate-evaluation"
	RoleResult         ArtifactRole = "gate-result"
	RoleEvidence       ArtifactRole = "evidence"
	RoleCounterexample ArtifactRole = "counterexample"
	RoleSupporting     ArtifactRole = "supporting"
)

// Descriptor is local assembly input. Paths are relative to the descriptor's
// root and are never trusted until bounded path and file checks succeed.
type Descriptor struct {
	Schema         string          `json:"schema"`
	SchemaVersion  string          `json:"schema_version"`
	Name           string          `json:"name"`
	CreatedAt      string          `json:"created_at"`
	EvaluationPath string          `json:"evaluation_path"`
	ResultPath     string          `json:"result_path"`
	Artifacts      []ArtifactInput `json:"artifacts"`
	Limitations    []string        `json:"limitations"`
}

type ArtifactInput struct {
	Name      string       `json:"name"`
	Role      ArtifactRole `json:"role"`
	Path      string       `json:"path"`
	MediaType string       `json:"media_type"`
}

type Manifest struct {
	Schema           string           `json:"schema"`
	SchemaVersion    string           `json:"schema_version"`
	Name             string           `json:"name"`
	CreatedAt        string           `json:"created_at"`
	ChangeDigest     string           `json:"change_digest"`
	EvaluationDigest string           `json:"evaluation_digest"`
	ResultDigest     string           `json:"result_digest"`
	Decision         gate.Decision    `json:"decision"`
	Artifacts        []ArtifactRecord `json:"artifacts"`
	Claims           []Claim          `json:"claims"`
	Gaps             []string         `json:"gaps"`
	Limitations      []string         `json:"limitations"`
}

type ArtifactRecord struct {
	Name      string       `json:"name"`
	Role      ArtifactRole `json:"role"`
	Path      string       `json:"path"`
	MediaType string       `json:"media_type"`
	Digest    string       `json:"digest"`
	SizeBytes uint64       `json:"size_bytes"`
}

// Claim is an exact projection of one gate finding, not a rewritten summary.
type Claim struct {
	RuleID         string              `json:"rule_id"`
	Outcome        gate.FindingOutcome `json:"outcome"`
	Code           gate.FindingCode    `json:"code"`
	EvidenceDigest string              `json:"evidence_digest,omitempty"`
	Message        string              `json:"message"`
}

type Signature struct {
	Schema         string `json:"schema"`
	SchemaVersion  string `json:"schema_version"`
	Algorithm      string `json:"algorithm"`
	ManifestDigest string `json:"manifest_digest"`
	KeyID          string `json:"key_id"`
	Value          string `json:"value"`
}
