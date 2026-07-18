package main

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingWriter struct{ err error }

func (writer failingWriter) Write([]byte) (int, error) { return 0, writer.err }

type shortWriter struct{}

func (shortWriter) Write(data []byte) (int, error) { return len(data) / 2, nil }

func TestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "help", args: []string{"help"}, wantCode: 0, wantStdout: "Usage:"},
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "inferlab dev"},
		{name: "policies", args: []string{"policies"}, wantCode: 0, wantStdout: "round-robin"},
		{name: "adapter usage", args: []string{"adapter"}, wantCode: 2, wantStderr: "inferlab adapter list"},
		{name: "adapter list", args: []string{"adapter", "list"}, wantCode: 0, wantStdout: "guidellm-fixture-v1"},
		{name: "adapter capabilities", args: []string{"adapter", "capabilities", "predicted-json-v1"}, wantCode: 0, wantStdout: "inferlab-predicted-metrics-v1"},
		{name: "unknown adapter", args: []string{"adapter", "capabilities", "missing"}, wantCode: 1, wantStderr: "unknown adapter"},
		{name: "change usage", args: []string{"change"}, wantCode: 2, wantStderr: "inferlab change validate"},
		{name: "evidence usage", args: []string{"evidence"}, wantCode: 2, wantStderr: "inferlab evidence validate"},
		{name: "gate usage", args: []string{"gate"}, wantCode: 2, wantStderr: "inferlab gate evaluate"},
		{name: "evaluate usage", args: []string{"evaluate"}, wantCode: 2, wantStderr: "inferlab evaluate"},
		{name: "runtime usage", args: []string{"runtime"}, wantCode: 2, wantStderr: "inferlab runtime validate"},
		{name: "safety case usage", args: []string{"safety-case"}, wantCode: 2, wantStderr: "inferlab safety-case assemble"},
		{name: "missing command", wantCode: 2, wantStderr: "Usage:"},
		{name: "unknown command", args: []string{"nope"}, wantCode: 2, wantStderr: "unknown command"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Fatalf("run() code = %d, want %d", got, tt.wantCode)
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout %q does not contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunReportsOutputFailure(t *testing.T) {
	t.Parallel()
	failure := failingWriter{err: errors.New("broken output")}
	if code := run([]string{"help"}, failure, &bytes.Buffer{}); code != 1 {
		t.Fatalf("stdout failure code = %d, want 1", code)
	}
	if code := run(nil, &bytes.Buffer{}, failure); code != 1 {
		t.Fatalf("stderr failure code = %d, want 1", code)
	}
	if code := run([]string{"help"}, shortWriter{}, &bytes.Buffer{}); code != 1 {
		t.Fatalf("short stdout code = %d, want 1", code)
	}
}

func TestRunGateInconclusive(t *testing.T) {
	t.Parallel()
	example := filepath.Join("..", "..", "examples", "missing-evidence-gate.json")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"gate", "evaluate", example}, &stdout, &stderr); got != 4 {
		t.Fatalf("run() code = %d, want 4; stderr=%q", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"decision":"INCONCLUSIVE"`) || !strings.Contains(stdout.String(), `"code":"missing-coverage"`) {
		t.Fatalf("stdout does not contain fail-closed result: %q", stdout.String())
	}
	resultPath := filepath.Join(t.TempDir(), "result.json")
	if err := os.WriteFile(resultPath, stdout.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"validate", "digest"} {
		stdout.Reset()
		stderr.Reset()
		if got := run([]string{"gate", "result", action, resultPath}, &stdout, &stderr); got != 0 {
			t.Fatalf("gate result %s code = %d, want 0; stderr=%q", action, got, stderr.String())
		}
	}
	stdout.Reset()
	stderr.Reset()
	if got := run([]string{"gate", "evaluation", "validate", example}, &stdout, &stderr); got != 0 {
		t.Fatalf("gate evaluation validate code = %d, want 0; stderr=%q", got, stderr.String())
	}
}

func TestRunEvaluateAliasWritesResult(t *testing.T) {
	t.Parallel()
	example := filepath.Join("..", "..", "examples", "missing-evidence-gate.json")
	result := filepath.Join(t.TempDir(), "result.json")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"evaluate", example, result}, &stdout, &stderr); got != 4 {
		t.Fatalf("run() code = %d, want 4; stderr=%q", got, stderr.String())
	}
	data, err := os.ReadFile(result)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"decision":"INCONCLUSIVE"`) {
		t.Fatalf("result does not contain decision: %s", data)
	}
}

func TestRunEvidenceAndRuntime(t *testing.T) {
	t.Parallel()
	root := filepath.Join("..", "..", "examples")
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "validate evidence", args: []string{"evidence", "validate", filepath.Join(root, "guidellm-observed-evidence.json")}, wantCode: 0, wantStdout: "valid guidellm-observation sha256:"},
		{name: "digest evidence", args: []string{"evidence", "digest", filepath.Join(root, "guidellm-observed-evidence.json")}, wantCode: 0, wantStdout: "sha256:"},
		{name: "validate runtime", args: []string{"runtime", "validate", filepath.Join(root, "runtime-signature-l4-vllm.json")}, wantCode: 0, wantStdout: "observed unknown=0"},
		{name: "digest runtime", args: []string{"runtime", "digest", filepath.Join(root, "runtime-signature-l4-vllm.json")}, wantCode: 0, wantStdout: "sha256:"},
		{name: "missing evidence", args: []string{"evidence", "validate", "missing.json"}, wantCode: 1, wantStderr: "open evidence envelope"},
		{name: "missing runtime", args: []string{"runtime", "validate", "missing.json"}, wantCode: 1, wantStderr: "open runtime signature"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Fatalf("run() code = %d, want %d; stderr=%q", got, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout %q does not contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunAdapterNormalize(t *testing.T) {
	t.Parallel()
	example := filepath.Join("..", "..", "examples", "guidellm-adapter-input.json")
	var stdout, stderr bytes.Buffer
	if got := run([]string{"adapter", "normalize", "guidellm-fixture-v1", example}, &stdout, &stderr); got != 0 {
		t.Fatalf("run() code = %d, want 0; stderr=%q", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"schema":"inferlab.normalized-report"`) || !strings.Contains(stdout.String(), `"value":812.5`) {
		t.Fatalf("stdout does not contain normalized evidence: %q", stdout.String())
	}
	report := filepath.Join(t.TempDir(), "normalized.json")
	if err := os.WriteFile(report, stdout.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"validate", "digest"} {
		stdout.Reset()
		stderr.Reset()
		if got := run([]string{"adapter", action, report}, &stdout, &stderr); got != 0 {
			t.Fatalf("adapter %s code = %d, want 0; stderr=%q", action, got, stderr.String())
		}
		if !strings.Contains(stdout.String(), "sha256:") {
			t.Fatalf("adapter %s stdout = %q, want digest", action, stdout.String())
		}
	}
}

func TestRunSafetyCaseLifecycle(t *testing.T) {
	t.Parallel()

	work := t.TempDir()
	copyTree(t, filepath.Join("..", "..", "examples"), work)
	result := filepath.Join(work, "block-result.json")
	manifest := filepath.Join(work, "block-safety-case.json")
	privateKey := filepath.Join(work, "private.pem")
	publicKey := filepath.Join(work, "public.pem")
	signature := filepath.Join(work, "block-safety-case.sig.json")

	commands := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
	}{
		{name: "evaluate block", args: []string{"evaluate", filepath.Join(work, "block-gate.json"), result}, wantCode: 3, wantStdout: "BLOCK "},
		{name: "assemble", args: []string{"safety-case", "assemble", filepath.Join(work, "block-safety-case-descriptor.json"), manifest}, wantStdout: "BLOCK sha256:"},
		{name: "validate", args: []string{"safety-case", "validate", manifest}, wantStdout: "valid public-synthetic-block-case sha256:"},
		{name: "digest", args: []string{"safety-case", "digest", manifest}, wantStdout: "sha256:"},
		{name: "keygen", args: []string{"safety-case", "keygen", privateKey, publicKey}, wantStdout: "sha256:"},
		{name: "sign", args: []string{"safety-case", "sign", manifest, privateKey, signature}, wantStdout: "sha256:"},
		{name: "verify default root", args: []string{"safety-case", "verify", manifest, signature, publicKey}, wantStdout: "verified public-synthetic-block-case"},
		{name: "verify explicit root", args: []string{"safety-case", "verify", manifest, signature, publicKey, work}, wantStdout: "verified public-synthetic-block-case"},
	}
	for _, tt := range commands {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Fatalf("run() code = %d, want %d; stderr=%q", got, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Fatalf("stdout %q does not contain %q", stdout.String(), tt.wantStdout)
			}
		})
	}

	info, err := os.Stat(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("private key mode = %o, want 600", info.Mode().Perm())
	}
}

func TestRunSafetyCaseFailures(t *testing.T) {
	t.Parallel()

	work := t.TempDir()
	copyTree(t, filepath.Join("..", "..", "examples"), work)
	invalidJSON := filepath.Join(work, "invalid.json")
	if err := os.WriteFile(invalidJSON, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	oversized := filepath.Join(work, "oversized.pem")
	if err := os.WriteFile(oversized, bytes.Repeat([]byte("x"), (64<<10)+1), 0o600); err != nil {
		t.Fatal(err)
	}
	privateKey := filepath.Join(work, "private.pem")
	publicKey := filepath.Join(work, "public.pem")
	if code := run([]string{"safety-case", "keygen", privateKey, publicKey}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("keygen setup code = %d, want 0", code)
	}
	manifest := filepath.Join(work, "block-safety-case.json")
	result := filepath.Join(work, "block-result.json")
	if code := run([]string{"evaluate", filepath.Join(work, "block-gate.json"), result}, &bytes.Buffer{}, &bytes.Buffer{}); code != 3 {
		t.Fatalf("evaluate setup code = %d, want 3", code)
	}
	if code := run([]string{"safety-case", "assemble", filepath.Join(work, "block-safety-case-descriptor.json"), manifest}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("assemble setup code = %d, want 0", code)
	}
	signature := filepath.Join(work, "valid.sig.json")
	if code := run([]string{"safety-case", "sign", manifest, privateKey, signature}, &bytes.Buffer{}, &bytes.Buffer{}); code != 0 {
		t.Fatalf("sign setup code = %d, want 0", code)
	}

	tests := []struct {
		name     string
		args     []string
		wantCode int
		wantErr  string
	}{
		{name: "assemble missing descriptor", args: []string{"safety-case", "assemble", filepath.Join(work, "missing.json"), filepath.Join(work, "out.json")}, wantCode: 1, wantErr: "assemble safety case"},
		{name: "assemble invalid descriptor", args: []string{"safety-case", "assemble", invalidJSON, filepath.Join(work, "out.json")}, wantCode: 1, wantErr: "assemble safety case"},
		{name: "assemble output parent missing", args: []string{"safety-case", "assemble", filepath.Join(work, "block-safety-case-descriptor.json"), filepath.Join(work, "missing", "out.json")}, wantCode: 1, wantErr: "write safety case"},
		{name: "validate invalid manifest", args: []string{"safety-case", "validate", invalidJSON}, wantCode: 1, wantErr: "invalid safety case"},
		{name: "duplicate private key", args: []string{"safety-case", "keygen", privateKey, filepath.Join(work, "unused.pem")}, wantCode: 1, wantErr: "write private key"},
		{name: "public key parent missing", args: []string{"safety-case", "keygen", filepath.Join(work, "transient.pem"), filepath.Join(work, "missing", "public.pem")}, wantCode: 1, wantErr: "write public key"},
		{name: "sign missing manifest", args: []string{"safety-case", "sign", filepath.Join(work, "missing.json"), privateKey, filepath.Join(work, "sig.json")}, wantCode: 1, wantErr: "sign safety case"},
		{name: "sign oversized key", args: []string{"safety-case", "sign", manifest, oversized, filepath.Join(work, "sig.json")}, wantCode: 1, wantErr: "64 KiB"},
		{name: "sign invalid key", args: []string{"safety-case", "sign", manifest, invalidJSON, filepath.Join(work, "sig.json")}, wantCode: 1, wantErr: "sign safety case"},
		{name: "sign output parent missing", args: []string{"safety-case", "sign", manifest, privateKey, filepath.Join(work, "missing", "sig.json")}, wantCode: 1, wantErr: "write safety-case signature"},
		{name: "verify missing manifest", args: []string{"safety-case", "verify", filepath.Join(work, "missing.json"), invalidJSON, publicKey}, wantCode: 1, wantErr: "verify safety case"},
		{name: "verify missing signature", args: []string{"safety-case", "verify", manifest, filepath.Join(work, "missing.sig"), publicKey}, wantCode: 1, wantErr: "verify safety case"},
		{name: "verify invalid signature", args: []string{"safety-case", "verify", manifest, invalidJSON, publicKey}, wantCode: 1, wantErr: "verify safety case"},
		{name: "verify oversized public key", args: []string{"safety-case", "verify", manifest, signature, oversized}, wantCode: 1, wantErr: "64 KiB"},
		{name: "verify invalid public key", args: []string{"safety-case", "verify", manifest, signature, invalidJSON}, wantCode: 1, wantErr: "verify safety case"},
		{name: "verify missing artifact root", args: []string{"safety-case", "verify", manifest, signature, publicKey, filepath.Join(work, "missing-root")}, wantCode: 1, wantErr: "verify safety case"},
		{name: "usage", args: []string{"safety-case", "verify"}, wantCode: 2, wantErr: "Usage:"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Fatalf("run() code = %d, want %d; stdout=%q stderr=%q", got, tt.wantCode, stdout.String(), stderr.String())
			}
			if !strings.Contains(stderr.String(), tt.wantErr) {
				t.Fatalf("stderr %q does not contain %q", stderr.String(), tt.wantErr)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(work, "transient.pem")); !os.IsNotExist(err) {
		t.Fatalf("private key was not rolled back after public key failure: %v", err)
	}
}

func TestRunGateAndDocumentFailures(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	invalid := filepath.Join(root, "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	block := filepath.Join("..", "..", "examples", "block-gate.json")
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "block to stdout", args: []string{"gate", "evaluate", block}, wantCode: 3, wantStdout: `"decision":"BLOCK"`},
		{name: "evaluate missing input", args: []string{"evaluate", filepath.Join(root, "missing.json")}, wantCode: 1, wantStderr: "open gate evaluation"},
		{name: "evaluate invalid input", args: []string{"evaluate", invalid}, wantCode: 1, wantStderr: "invalid gate evaluation"},
		{name: "evaluate output parent missing", args: []string{"evaluate", block, filepath.Join(root, "missing", "result.json")}, wantCode: 1, wantStderr: "write gate result"},
		{name: "gate evaluation missing", args: []string{"gate", "evaluation", "validate", filepath.Join(root, "missing.json")}, wantCode: 1, wantStderr: "open gate evaluation"},
		{name: "gate evaluation invalid", args: []string{"gate", "evaluation", "validate", invalid}, wantCode: 1, wantStderr: "invalid gate evaluation"},
		{name: "gate result missing", args: []string{"gate", "result", "validate", filepath.Join(root, "missing.json")}, wantCode: 1, wantStderr: "open gate result"},
		{name: "gate result invalid", args: []string{"gate", "result", "digest", invalid}, wantCode: 1, wantStderr: "invalid gate result"},
		{name: "adapter normalize unknown", args: []string{"adapter", "normalize", "missing", invalid}, wantCode: 1, wantStderr: "unknown adapter"},
		{name: "adapter normalize missing", args: []string{"adapter", "normalize", "guidellm-fixture-v1", filepath.Join(root, "missing.json")}, wantCode: 1, wantStderr: "open adapter input"},
		{name: "adapter normalize invalid", args: []string{"adapter", "normalize", "guidellm-fixture-v1", invalid}, wantCode: 1, wantStderr: "invalid adapter input"},
		{name: "adapter report missing", args: []string{"adapter", "validate", filepath.Join(root, "missing.json")}, wantCode: 1, wantStderr: "open normalized report"},
		{name: "adapter report invalid", args: []string{"adapter", "digest", invalid}, wantCode: 1, wantStderr: "invalid normalized report"},
		{name: "evidence invalid", args: []string{"evidence", "validate", invalid}, wantCode: 1, wantStderr: "invalid evidence envelope"},
		{name: "runtime invalid", args: []string{"runtime", "digest", invalid}, wantCode: 1, wantStderr: "invalid runtime signature"},
		{name: "change invalid", args: []string{"change", "validate", invalid}, wantCode: 1, wantStderr: "invalid inference change"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Fatalf("run() code = %d, want %d; stdout=%q stderr=%q", got, tt.wantCode, stdout.String(), stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Fatalf("stdout %q does not contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Fatalf("stderr %q does not contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestFileHelpers(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := filepath.Join(root, "artifact.json")
	if err := atomicWrite(path, []byte("first"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(path, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "second" {
		t.Fatalf("atomic output = %q, %v", data, err)
	}
	if err := atomicWrite(filepath.Join(root, "missing", "out"), nil, 0o600); err == nil {
		t.Fatal("atomicWrite() accepted a missing parent")
	}

	exclusive := filepath.Join(root, "exclusive")
	if err := writeExclusive(exclusive, []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeExclusive(exclusive, nil, 0o600); !os.IsExist(err) {
		t.Fatalf("writeExclusive() error = %v, want exists", err)
	}
	if got, err := readSmallFile(exclusive); err != nil || string(got) != "secret" {
		t.Fatalf("readSmallFile() = %q, %v", got, err)
	}
	if _, err := readSmallFile(filepath.Join(root, "missing")); err == nil {
		t.Fatal("readSmallFile() accepted a missing file")
	}
	large := filepath.Join(root, "large")
	if err := os.WriteFile(large, bytes.Repeat([]byte("x"), (64<<10)+1), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSmallFile(large); err == nil || !strings.Contains(err.Error(), "64 KiB") {
		t.Fatalf("readSmallFile() error = %v, want size limit", err)
	}
}

func copyTree(t *testing.T, source, destination string) {
	t.Helper()
	if err := filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o600)
	}); err != nil {
		t.Fatal(err)
	}
}

func TestRunChange(t *testing.T) {
	t.Parallel()

	example := filepath.Join("..", "..", "examples", "qwen-vllm-batching-change.json")
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "validate", args: []string{"change", "validate", example}, wantCode: 0, wantStdout: "valid qwen-vllm-batching"},
		{name: "digest", args: []string{"change", "digest", example}, wantCode: 0, wantStdout: "sha256:"},
		{name: "missing file", args: []string{"change", "validate", "missing.json"}, wantCode: 1, wantStderr: "open inference change"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			if got := run(tt.args, &stdout, &stderr); got != tt.wantCode {
				t.Fatalf("run() code = %d, want %d; stderr=%q", got, tt.wantCode, stderr.String())
			}
			if !strings.Contains(stdout.String(), tt.wantStdout) {
				t.Errorf("stdout %q does not contain %q", stdout.String(), tt.wantStdout)
			}
			if !strings.Contains(stderr.String(), tt.wantStderr) {
				t.Errorf("stderr %q does not contain %q", stderr.String(), tt.wantStderr)
			}
		})
	}
}

func TestRunChangeRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	data, err := os.ReadFile(filepath.Join("..", "..", "examples", "qwen-vllm-batching-change.json"))
	if err != nil {
		t.Fatal(err)
	}
	invalid := append(bytes.TrimSuffix(data, []byte("\n")), []byte("\ntrue")...)
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, invalid, 0o600); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if got := run([]string{"change", "validate", path}, &stdout, &stderr); got != 1 {
		t.Fatalf("run() code = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "trailing JSON value") {
		t.Fatalf("stderr %q does not describe the validation error", stderr.String())
	}
}
