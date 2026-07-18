package strictjson

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type errorReader struct{ err error }

func (reader errorReader) Read([]byte) (int, error) { return 0, reader.err }

func TestReadOne(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		data    []byte
		max     int64
		wantErr error
	}{
		{name: "valid", data: []byte(`{"nested":[1,true,null]}`), max: 128},
		{name: "duplicate", data: []byte(`{"a":1,"a":2}`), max: 128, wantErr: ErrDuplicateField},
		{name: "trailing", data: []byte(`{} {}`), max: 128, wantErr: errors.New("multiple JSON values")},
		{name: "too large", data: []byte(`{"value":true}`), max: 2, wantErr: ErrTooLarge},
		{name: "malformed", data: []byte(`{"value":`), max: 128, wantErr: errors.New("decode JSON token")},
		{name: "malformed trailing", data: []byte(`{} !`), max: 128, wantErr: errors.New("trailing JSON")},
		{name: "deep nesting", data: []byte(strings.Repeat("[", MaxDepth+2) + strings.Repeat("]", MaxDepth+2)), max: 1024, wantErr: ErrNestingLimit},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ReadOne(bytes.NewReader(tt.data), tt.max)
			if tt.wantErr == nil && err != nil {
				t.Fatal(err)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) && !bytes.Contains([]byte(err.Error()), []byte(tt.wantErr.Error())) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestReadOneRejectsInvalidReaderAndLimit(t *testing.T) {
	t.Parallel()
	if _, err := ReadOne(nil, 1); err == nil {
		t.Fatal("ReadOne() accepted a nil reader")
	}
	if _, err := ReadOne(bytes.NewReader(nil), 0); err == nil {
		t.Fatal("ReadOne() accepted a zero limit")
	}
	want := errors.New("read failure")
	if _, err := ReadOne(errorReader{err: want}, 1); !errors.Is(err, want) {
		t.Fatalf("ReadOne() error = %v, want %v", err, want)
	}
	if _, err := ReadOne(errorReader{err: io.ErrUnexpectedEOF}, 1); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("ReadOne() error = %v, want unexpected EOF", err)
	}
}
