package change

import (
	"bytes"
	"encoding/json"
	"testing"
)

func FuzzDecoderNeverPanics(f *testing.F) {
	encoded, err := json.Marshal(validDocument())
	if err != nil {
		f.Fatal(err)
	}
	f.Add(encoded)
	f.Add([]byte(`{"schema":"inferlab.change"}`))
	f.Add([]byte{})

	f.Fuzz(func(t *testing.T, input []byte) {
		if len(input) > MaxDocumentBytes+1 {
			input = input[:MaxDocumentBytes+1]
		}
		_, _ = Decode(bytes.NewReader(input))
	})
}
