package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

// Runner invokes one adapter executable directly, without a shell. Its
// default environment is empty; callers must explicitly opt into inheritance.
type Runner struct {
	Executable         string
	Arguments          []string
	Directory          string
	Environment        []string
	InheritEnvironment bool
	MaxOutputBytes     int64
	WaitDelay          time.Duration
}

func (runner Runner) Invoke(ctx context.Context, request Request) (Response, error) {
	if ctx == nil {
		return Response{}, fmt.Errorf("%w: context is nil", ErrProtocol)
	}
	if err := ValidateRequest(request); err != nil {
		return Response{}, err
	}
	if runner.Executable == "" {
		return Response{}, fmt.Errorf("%w: executable is empty", ErrProtocol)
	}
	encoded, err := json.Marshal(request)
	if err != nil {
		return Response{}, fmt.Errorf("%w: encode request: %v", ErrProtocol, err)
	}
	if len(encoded) > MaxInputBytes {
		return Response{}, fmt.Errorf("%w: encoded request exceeds %d bytes", ErrProtocol, MaxInputBytes)
	}

	limit := runner.MaxOutputBytes
	if limit == 0 {
		limit = DefaultMaxOutputBytes
	}
	if limit < 1 || limit > 64<<20 {
		return Response{}, fmt.Errorf("%w: output limit must be between 1 and 67108864 bytes", ErrProtocol)
	}
	waitDelay := runner.WaitDelay
	if waitDelay == 0 {
		waitDelay = 2 * time.Second
	}
	if waitDelay < 0 || waitDelay > 30*time.Second {
		return Response{}, fmt.Errorf("%w: wait delay must be between zero and 30 seconds", ErrProtocol)
	}

	command := exec.CommandContext(ctx, runner.Executable, runner.Arguments...)
	command.Dir = runner.Directory
	if runner.InheritEnvironment {
		command.Env = append(os.Environ(), runner.Environment...)
	} else {
		command.Env = slices.Clone(runner.Environment)
		if command.Env == nil {
			command.Env = []string{}
		}
	}
	command.Stdin = bytes.NewReader(append(encoded, '\n'))
	stdout := &limitedBuffer{limit: limit}
	stderr := &limitedBuffer{limit: 64 << 10}
	command.Stdout = stdout
	command.Stderr = stderr
	command.WaitDelay = waitDelay

	err = command.Run()
	if stdout.exceeded {
		return Response{}, ErrOutputLimit
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return Response{}, ctxErr
	}
	if err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return Response{}, fmt.Errorf("%w: %s", ErrAdapterFailed, message)
	}
	response, err := DecodeResponse(bytes.NewReader(stdout.Bytes()), limit)
	if err != nil {
		return Response{}, err
	}
	if response.RequestID != request.RequestID {
		return Response{}, fmt.Errorf("%w: response request_id %q does not match %q", ErrProtocol, response.RequestID, request.RequestID)
	}
	if response.Failure != nil {
		return Response{}, fmt.Errorf("%w: %s: %s", ErrAdapterFailed, response.Failure.Code, response.Failure.Message)
	}
	if request.Operation == OperationCapabilities && response.Capabilities == nil {
		return Response{}, fmt.Errorf("%w: capabilities request returned another payload", ErrProtocol)
	}
	if request.Operation == OperationNormalize && response.Report == nil {
		return Response{}, fmt.Errorf("%w: normalize request returned another payload", ErrProtocol)
	}
	return response, nil
}

type limitedBuffer struct {
	buffer   bytes.Buffer
	limit    int64
	exceeded bool
}

func (buffer *limitedBuffer) Write(data []byte) (int, error) {
	remaining := buffer.limit - int64(buffer.buffer.Len())
	if remaining <= 0 {
		buffer.exceeded = true
		return 0, ErrOutputLimit
	}
	if int64(len(data)) > remaining {
		written, _ := buffer.buffer.Write(data[:remaining])
		buffer.exceeded = true
		return written, ErrOutputLimit
	}
	return buffer.buffer.Write(data)
}

func (buffer *limitedBuffer) Bytes() []byte  { return buffer.buffer.Bytes() }
func (buffer *limitedBuffer) String() string { return buffer.buffer.String() }
