package evidence

import (
	"bytes"
	"encoding/json"
	"testing"
)

func FuzzDecoderNeverPanics(f *testing.F) {
	encoded, err := json.Marshal(validEnvelope())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(encoded)
	f.Add([]byte(`{"schema":"inferlab.evidence"}`))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > MaxDocumentBytes+1 {
			input = input[:MaxDocumentBytes+1]
		}
		_, _ = Decode(bytes.NewReader(input))
	})
}

func FuzzRuntimeDecoderNeverPanics(f *testing.F) {
	encoded, err := json.Marshal(validRuntimeSignature(OriginObserved))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(encoded)
	f.Add([]byte(`{"schema":"inferlab.runtime-signature"}`))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > MaxDocumentBytes+1 {
			input = input[:MaxDocumentBytes+1]
		}
		_, _ = DecodeRuntimeSignature(bytes.NewReader(input))
	})
}
