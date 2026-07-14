package trace

import (
	"bytes"
	"errors"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestCanonicalGolden(t *testing.T) {
	t.Parallel()

	got, err := MarshalCanonical(validRecord())
	if err != nil {
		t.Fatalf("MarshalCanonical() error: %v", err)
	}
	got = append(got, '\n')
	want, err := os.ReadFile("testdata/record_v1.jsonl")
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("canonical record mismatch\ngot:  %s\nwant: %s", got, want)
	}
}

func TestCodecRoundTrip(t *testing.T) {
	t.Parallel()

	first := validRecord()
	second := validRecord()
	second.Sequence = 2
	second.RequestID = "req-0002"
	second.Metadata = map[string]string{"region": "us-east-1"}

	var output bytes.Buffer
	encoder := NewEncoder(&output, Limits{})
	for _, record := range []Record{first, second} {
		if err := encoder.Encode(record); err != nil {
			t.Fatalf("Encode() error: %v", err)
		}
	}

	decoder := NewDecoder(bytes.NewReader(output.Bytes()), Limits{})
	for i, want := range []Record{first, second} {
		got, err := decoder.Decode()
		if err != nil {
			t.Fatalf("Decode() record %d error: %v", i+1, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Decode() record %d = %#v, want %#v", i+1, got, want)
		}
	}
	if _, err := decoder.Decode(); !errors.Is(err, io.EOF) {
		t.Fatalf("Decode() final error = %v, want io.EOF", err)
	}
}

func TestDecoderForwardCompatibility(t *testing.T) {
	t.Parallel()

	encoded, err := MarshalCanonical(validRecord())
	if err != nil {
		t.Fatalf("MarshalCanonical() error: %v", err)
	}
	future := strings.Replace(string(encoded), `"schema_version":"1.0"`, `"schema_version":"1.99"`, 1)
	future = strings.TrimSuffix(future, "}") + `,"future_safe":{"enabled":true}}` + "\n"

	record, err := NewDecoder(strings.NewReader(future), Limits{}).Decode()
	if err != nil {
		t.Fatalf("Decode() future-minor record error: %v", err)
	}
	if record.SchemaVersion != "1.99" {
		t.Fatalf("SchemaVersion = %q, want 1.99", record.SchemaVersion)
	}
	if _, err := MarshalCanonical(record); !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("MarshalCanonical() future-minor error = %v, want ErrUnsupportedVersion", err)
	}
}

func TestDecoderRejectsUnsafeJSON(t *testing.T) {
	t.Parallel()

	encoded, err := MarshalCanonical(validRecord())
	if err != nil {
		t.Fatalf("MarshalCanonical() error: %v", err)
	}
	base := strings.TrimSuffix(string(encoded), "}")
	tests := []struct {
		name string
		data []byte
		want error
	}{
		{name: "raw prompt", data: []byte(base + `,"prompt":"secret"}`), want: ErrSensitiveField},
		{name: "nested content", data: []byte(base + `,"future":{"content":"secret"}}`), want: ErrSensitiveField},
		{name: "duplicate field", data: []byte(base + `,"model":"other"}`), want: ErrDuplicateField},
		{name: "unsupported major", data: []byte(strings.Replace(string(encoded), `"schema_version":"1.0"`, `"schema_version":"2.0"`, 1)), want: ErrUnsupportedVersion},
		{name: "invalid UTF-8", data: append(append([]byte(nil), encoded[:len(encoded)-1]...), []byte{',', '"', 'x', '"', ':', '"', 0xff, '"', '}'}...), want: ErrInvalidRecord},
		{name: "multiple values", data: append(append([]byte(nil), encoded...), []byte(` {}`)...), want: ErrInvalidRecord},
		{name: "empty line", data: []byte("\n"), want: ErrInvalidRecord},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			decoder := NewDecoder(bytes.NewReader(tt.data), Limits{})
			_, got := decoder.Decode()
			if !errors.Is(got, tt.want) {
				t.Fatalf("Decode() error = %v, want %v", got, tt.want)
			}
			var location *DecodeError
			if !errors.As(got, &location) || location.Record != 1 || location.ByteOffset != 0 {
				t.Fatalf("Decode() location = %#v, want record 1 byte 0", location)
			}
			if _, repeated := decoder.Decode(); repeated != got {
				t.Fatalf("terminal decoder error changed: first %p, second %p", got, repeated)
			}
		})
	}
}

func TestDecodeErrorReportsRecordAndOffset(t *testing.T) {
	t.Parallel()

	valid, err := MarshalCanonical(validRecord())
	if err != nil {
		t.Fatalf("MarshalCanonical() error: %v", err)
	}
	input := append(append(append([]byte(nil), valid...), '\n'), []byte("not-json\n")...)
	decoder := NewDecoder(bytes.NewReader(input), Limits{})
	if _, err := decoder.Decode(); err != nil {
		t.Fatalf("Decode() first record error: %v", err)
	}
	_, err = decoder.Decode()
	var location *DecodeError
	if !errors.As(err, &location) {
		t.Fatalf("Decode() error = %v, want DecodeError", err)
	}
	if location.Record != 2 || location.ByteOffset != int64(len(valid)+1) {
		t.Fatalf("DecodeError = %#v, want record 2 byte %d", location, len(valid)+1)
	}
}

func TestDecoderLimits(t *testing.T) {
	t.Parallel()

	encoded, err := MarshalCanonical(validRecord())
	if err != nil {
		t.Fatalf("MarshalCanonical() error: %v", err)
	}

	t.Run("record bytes", func(t *testing.T) {
		decoder := NewDecoder(bytes.NewReader(encoded), Limits{MaxRecordBytes: 32})
		if _, err := decoder.Decode(); !errors.Is(err, ErrRecordTooLarge) {
			t.Fatalf("Decode() error = %v, want ErrRecordTooLarge", err)
		}
	})

	t.Run("trace bytes", func(t *testing.T) {
		decoder := NewDecoder(bytes.NewReader(encoded), Limits{MaxTraceBytes: int64(len(encoded) - 1)})
		if _, err := decoder.Decode(); !errors.Is(err, ErrTraceTooLarge) {
			t.Fatalf("Decode() error = %v, want ErrTraceTooLarge", err)
		}
	})

	t.Run("record count", func(t *testing.T) {
		input := append(append(append([]byte(nil), encoded...), '\n'), encoded...)
		decoder := NewDecoder(bytes.NewReader(input), Limits{MaxRecords: 1})
		if _, err := decoder.Decode(); err != nil {
			t.Fatalf("Decode() first error: %v", err)
		}
		if _, err := decoder.Decode(); !errors.Is(err, ErrRecordLimit) {
			t.Fatalf("Decode() second error = %v, want ErrRecordLimit", err)
		}
	})

	t.Run("token count", func(t *testing.T) {
		decoder := NewDecoder(bytes.NewReader(encoded), Limits{MaxInputTokens: 100})
		if _, err := decoder.Decode(); !errors.Is(err, ErrTokenLimit) {
			t.Fatalf("Decode() error = %v, want ErrTokenLimit", err)
		}
	})

	t.Run("nesting", func(t *testing.T) {
		deep := strings.TrimSuffix(string(encoded), "}") + `,"future":{"nested":true}}`
		decoder := NewDecoder(strings.NewReader(deep), Limits{MaxNestingDepth: 1})
		if _, err := decoder.Decode(); !errors.Is(err, ErrInvalidRecord) {
			t.Fatalf("Decode() error = %v, want ErrInvalidRecord", err)
		}
	})
}

func TestEncoderLimitsAndFailureState(t *testing.T) {
	t.Parallel()

	t.Run("record count", func(t *testing.T) {
		var output bytes.Buffer
		encoder := NewEncoder(&output, Limits{MaxRecords: 1})
		if err := encoder.Encode(validRecord()); err != nil {
			t.Fatalf("Encode() first error: %v", err)
		}
		if err := encoder.Encode(validRecord()); !errors.Is(err, ErrRecordLimit) {
			t.Fatalf("Encode() second error = %v, want ErrRecordLimit", err)
		}
	})

	t.Run("trace bytes", func(t *testing.T) {
		encoder := NewEncoder(io.Discard, Limits{MaxTraceBytes: 1})
		if err := encoder.Encode(validRecord()); !errors.Is(err, ErrTraceTooLarge) {
			t.Fatalf("Encode() error = %v, want ErrTraceTooLarge", err)
		}
	})

	t.Run("terminal writer error", func(t *testing.T) {
		want := errors.New("storage unavailable")
		encoder := NewEncoder(failingWriter{err: want}, Limits{})
		if err := encoder.Encode(validRecord()); !errors.Is(err, want) {
			t.Fatalf("Encode() first error = %v, want writer error", err)
		}
		if err := encoder.Encode(validRecord()); !errors.Is(err, ErrEncoderFailed) {
			t.Fatalf("Encode() repeated error = %v, want ErrEncoderFailed", err)
		}
	})

	t.Run("short writes", func(t *testing.T) {
		writer := &shortWriter{max: 3}
		if err := NewEncoder(writer, Limits{}).Encode(validRecord()); err != nil {
			t.Fatalf("Encode() error: %v", err)
		}
		if !bytes.HasSuffix(writer.data, []byte("\n")) {
			t.Fatal("encoded stream has no newline")
		}
	})
}

func TestDecoderAcceptsCRLFAndFinalLineWithoutNewline(t *testing.T) {
	t.Parallel()

	encoded, err := MarshalCanonical(validRecord())
	if err != nil {
		t.Fatalf("MarshalCanonical() error: %v", err)
	}
	for _, suffix := range []string{"", "\r\n"} {
		decoder := NewDecoder(bytes.NewReader(append(append([]byte(nil), encoded...), suffix...)), Limits{})
		if _, err := decoder.Decode(); err != nil {
			t.Fatalf("Decode() suffix %q error: %v", suffix, err)
		}
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

type shortWriter struct {
	max  int
	data []byte
}

func (w *shortWriter) Write(data []byte) (int, error) {
	count := min(w.max, len(data))
	w.data = append(w.data, data[:count]...)
	return count, nil
}
