package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
