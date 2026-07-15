package adapter

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
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
