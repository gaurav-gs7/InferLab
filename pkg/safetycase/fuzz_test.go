package safetycase

import (
	"bytes"
	"testing"
)

func FuzzDecoderNeverPanics(f *testing.F) {
	f.Add([]byte(`{"schema":"inferlab.safety-case","schema_version":"1.0"}`))
	f.Add([]byte(`{"a":1,"a":2}`))
	f.Add([]byte(`null`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeManifest(bytes.NewReader(data))
		_, _ = DecodeDescriptor(bytes.NewReader(data))
		_, _ = DecodeSignature(bytes.NewReader(data))
	})
}
