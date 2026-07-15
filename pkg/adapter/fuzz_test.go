package adapter

import (
	"bytes"
	"encoding/json"
	"testing"
)

func FuzzDecoderNeverPanics(f *testing.F) {
	seed, err := json.Marshal(testInput(GuideLLMAdapterName))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte(`{"schema":"inferlab.adapter-protocol"}`))
	f.Add([]byte(`{"a":1,"a":2}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodeInput(bytes.NewReader(data))
	})
}
