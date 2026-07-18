package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunnerCapabilitiesAndNormalization(t *testing.T) {
	t.Parallel()
	runner := helperRunner("serve", 0)

	capabilitiesResponse, err := runner.Invoke(context.Background(), Request{
		Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "capabilities-1", Operation: OperationCapabilities,
	})
	if err != nil {
		t.Fatal(err)
	}
	if capabilitiesResponse.Capabilities == nil || capabilitiesResponse.Capabilities.Adapter.Name != GuideLLMAdapterName {
		t.Fatalf("unexpected capabilities: %+v", capabilitiesResponse.Capabilities)
	}

	input := testInput(GuideLLMAdapterName)
	normalizeResponse, err := runner.Invoke(context.Background(), Request{
		Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "normalize-1", Operation: OperationNormalize, Input: &input,
	})
	if err != nil {
		t.Fatal(err)
	}
	if normalizeResponse.Report == nil || normalizeResponse.Report.Envelope.Classification != input.Classification {
		t.Fatalf("unexpected report: %+v", normalizeResponse.Report)
	}
}

func TestRunnerCancellationAndOutputBound(t *testing.T) {
	t.Parallel()
	request := Request{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "bounded-1", Operation: OperationCapabilities}

	t.Run("cancellation", func(t *testing.T) {
		t.Parallel()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		_, err := helperRunner("sleep", 0).Invoke(ctx, request)
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("error = %v, want deadline exceeded", err)
		}
	})
	t.Run("output limit", func(t *testing.T) {
		t.Parallel()
		_, err := helperRunner("overflow", 128).Invoke(context.Background(), request)
		if !errors.Is(err, ErrOutputLimit) {
			t.Fatalf("error = %v, want output limit", err)
		}
	})
	t.Run("nonzero exit", func(t *testing.T) {
		t.Parallel()
		_, err := helperRunner("fail", 0).Invoke(context.Background(), request)
		if !errors.Is(err, ErrAdapterFailed) || !strings.Contains(err.Error(), "fixture failure") {
			t.Fatalf("error = %v, want sanitized adapter failure", err)
		}
	})
}

func TestRunnerRejectsMismatchedResponse(t *testing.T) {
	t.Parallel()
	request := Request{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "expected-id", Operation: OperationCapabilities}
	_, err := helperRunner("mismatch", 0).Invoke(context.Background(), request)
	if !errors.Is(err, ErrProtocol) {
		t.Fatalf("error = %v, want protocol violation", err)
	}
}

func TestRunnerRejectsInvalidConfigurationAndPayloads(t *testing.T) {
	t.Parallel()
	request := Request{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: "request-1", Operation: OperationCapabilities}
	tests := []struct {
		name   string
		runner Runner
		ctx    context.Context
	}{
		{name: "nil context", runner: helperRunner("serve", 0)},
		{name: "empty executable", runner: Runner{}, ctx: context.Background()},
		{name: "negative output limit", runner: Runner{Executable: os.Args[0], MaxOutputBytes: -1}, ctx: context.Background()},
		{name: "excessive output limit", runner: Runner{Executable: os.Args[0], MaxOutputBytes: (64 << 20) + 1}, ctx: context.Background()},
		{name: "negative wait delay", runner: Runner{Executable: os.Args[0], WaitDelay: -1}, ctx: context.Background()},
		{name: "excessive wait delay", runner: Runner{Executable: os.Args[0], WaitDelay: 31 * time.Second}, ctx: context.Background()},
		{name: "missing executable", runner: Runner{Executable: filepath.Join(t.TempDir(), "missing")}, ctx: context.Background()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := tt.runner.Invoke(tt.ctx, request); err == nil {
				t.Fatal("Invoke() accepted invalid configuration")
			}
		})
	}
	if _, err := helperRunner("failure-response", 0).Invoke(context.Background(), request); !errors.Is(err, ErrAdapterFailed) {
		t.Fatalf("failure response error = %v, want %v", err, ErrAdapterFailed)
	}
	if _, err := helperRunner("wrong-payload", 0).Invoke(context.Background(), request); !errors.Is(err, ErrProtocol) {
		t.Fatalf("wrong payload error = %v, want %v", err, ErrProtocol)
	}
	invalid := request
	invalid.RequestID = "Bad ID"
	if _, err := helperRunner("serve", 0).Invoke(context.Background(), invalid); !errors.Is(err, ErrProtocol) {
		t.Fatalf("invalid request error = %v, want %v", err, ErrProtocol)
	}

	inherited := helperRunner("serve", 0)
	inherited.InheritEnvironment = true
	if _, err := inherited.Invoke(context.Background(), request); err != nil {
		t.Fatalf("inherited environment invocation: %v", err)
	}
}

func helperRunner(mode string, limit int64) Runner {
	return Runner{
		Executable:     os.Args[0],
		Arguments:      []string{"-test.run=TestAdapterHelperProcess", "--", mode},
		Environment:    []string{"INFERLAB_ADAPTER_HELPER=1"},
		MaxOutputBytes: limit,
	}
}

func TestAdapterHelperProcess(t *testing.T) {
	if os.Getenv("INFERLAB_ADAPTER_HELPER") != "1" {
		return
	}
	mode := ""
	for i, arg := range os.Args {
		if arg == "--" && i+1 < len(os.Args) {
			mode = os.Args[i+1]
		}
	}
	switch mode {
	case "sleep":
		time.Sleep(5 * time.Second)
	case "overflow":
		fmt.Print(strings.Repeat("x", 4096))
	case "fail":
		fmt.Fprintln(os.Stderr, "fixture failure")
		os.Exit(9)
	}
	request, err := DecodeRequest(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(10)
	}
	response := Response{Schema: ProtocolSchema, SchemaVersion: CurrentVersion, RequestID: request.RequestID}
	implementation, _ := Builtin(GuideLLMAdapterName)
	switch mode {
	case "mismatch":
		response.RequestID = "different-id"
		capabilities := implementation.Capabilities()
		response.Capabilities = &capabilities
	case "failure-response":
		response.Failure = &Failure{Code: "producer-failed", Message: "bounded failure"}
	case "wrong-payload":
		input := testInput(GuideLLMAdapterName)
		report, normalizeErr := implementation.Normalize(input)
		if normalizeErr != nil {
			os.Exit(11)
		}
		response.Report = &report
	case "serve":
		if request.Operation == OperationCapabilities {
			capabilities := implementation.Capabilities()
			response.Capabilities = &capabilities
		} else {
			report, normalizeErr := implementation.Normalize(*request.Input)
			if normalizeErr != nil {
				fmt.Fprintln(os.Stderr, normalizeErr)
				os.Exit(11)
			}
			response.Report = &report
		}
	default:
		os.Exit(12)
	}
	if err := json.NewEncoder(os.Stdout).Encode(response); err != nil {
		os.Exit(13)
	}
	os.Exit(0)
}
