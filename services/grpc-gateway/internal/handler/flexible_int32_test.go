package handler

import (
	"encoding/json"
	"testing"
)

func TestFlexibleInt32_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int32
		wantErr bool
	}{
		{name: "integer", input: `15`, want: 15},
		{name: "string", input: `"15"`, want: 15},
		{name: "persian string", input: `"۱۵"`, want: 15},
		{name: "float", input: `15.0`, want: 15},
		{name: "null", input: `null`, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got flexibleInt32
			err := json.Unmarshal([]byte(tt.input), &got)
			if (err != nil) != tt.wantErr {
				t.Fatalf("UnmarshalJSON() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got.Int32() != tt.want {
				t.Fatalf("got %d, want %d", got.Int32(), tt.want)
			}
		})
	}
}

func TestAccountSecurityRequestTimeField(t *testing.T) {
	var req struct {
		Time flexibleInt32 `json:"time"`
	}

	if err := json.Unmarshal([]byte(`{"time":15}`), &req); err != nil {
		t.Fatalf("unmarshal time: %v", err)
	}
	if req.Time.Int32() != 15 {
		t.Fatalf("expected 15 minutes, got %d", req.Time.Int32())
	}

	if err := json.Unmarshal([]byte(`{"time":"15"}`), &req); err != nil {
		t.Fatalf("unmarshal time string: %v", err)
	}
	if req.Time.Int32() != 15 {
		t.Fatalf("expected 15 minutes from string time")
	}
}
