package strictjson

import (
	"bytes"
	"errors"
	"testing"
)

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
