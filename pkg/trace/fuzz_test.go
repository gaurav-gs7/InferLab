package trace

import (
	"bytes"
	"testing"
)

func FuzzDecoderNeverPanics(f *testing.F) {
	canonical, err := MarshalCanonical(validRecord())
	if err != nil {
		f.Fatalf("MarshalCanonical() error: %v", err)
	}
	f.Add(canonical)
	f.Add([]byte(`{"schema":"inferlab.trace"}`))
	f.Add([]byte("\xff\n"))
	f.Add([]byte("\n"))

	f.Fuzz(func(t *testing.T, data []byte) {
		limits := Limits{
			MaxRecordBytes:  4 << 10,
			MaxTraceBytes:   8 << 10,
			MaxRecords:      4,
			MaxNestingDepth: 8,
		}
		decoder := NewDecoder(bytes.NewReader(data), limits)
		for range limits.MaxRecords + 1 {
			_, err := decoder.Decode()
			if err != nil {
				return
			}
		}
	})
}
