package models_test

import (
	"encoding/json"
	"testing"

	"metarang/auth-service/internal/models"
)

func TestParsePrivacyJSON(t *testing.T) {
	t.Run("empty and invalid fall back to defaults", func(t *testing.T) {
		for _, raw := range []string{"", "   ", "{bad", "null", "[]"} {
			got := models.ParsePrivacyJSON(raw)
			if got["name"] != 1 {
				t.Fatalf("raw=%q expected default name=1, got %d", raw, got["name"])
			}
			if got["phone"] != 0 {
				t.Fatalf("raw=%q expected default phone=0, got %d", raw, got["phone"])
			}
		}
	})

	t.Run("integer JSON after privacy update keeps unrelated public fields", func(t *testing.T) {
		// This is the payload written by UpdatePrivacySettings (map[string]int).
		raw := `{"phone":1,"name":1,"email":0,"level":1,"score":1}`
		got := models.ParsePrivacyJSON(raw)
		if got["phone"] != 1 {
			t.Fatalf("phone=%d", got["phone"])
		}
		if got["email"] != 0 {
			t.Fatalf("email=%d", got["email"])
		}
		if got["name"] != 1 {
			t.Fatalf("name=%d", got["name"])
		}
		if got["occupation"] != 1 {
			t.Fatalf("occupation should keep default 1, got %d", got["occupation"])
		}
		if got["lname"] != 1 {
			t.Fatalf("lname should keep default 1, got %d", got["lname"])
		}
	})

	t.Run("legacy boolean JSON overlays defaults", func(t *testing.T) {
		raw := `{"score":true,"phone":false,"name":true}`
		got := models.ParsePrivacyJSON(raw)
		if got["score"] != 1 || got["phone"] != 0 || got["name"] != 1 {
			t.Fatalf("got=%v", got)
		}
		if got["level"] != 1 {
			t.Fatalf("level default lost: %d", got["level"])
		}
	})

	t.Run("mixed bool number and string values", func(t *testing.T) {
		payload, err := json.Marshal(map[string]interface{}{
			"score": true,
			"name":  1,
			"email": "true",
			"phone": "0",
			"level": 2.0,
		})
		if err != nil {
			t.Fatal(err)
		}
		got := models.ParsePrivacyJSON(string(payload))
		if got["score"] != 1 || got["name"] != 1 || got["email"] != 1 || got["phone"] != 0 || got["level"] != 1 {
			t.Fatalf("got=%v", map[string]int{
				"score": got["score"], "name": got["name"], "email": got["email"],
				"phone": got["phone"], "level": got["level"],
			})
		}
	})
}

func TestPrivacyValueToInt(t *testing.T) {
	cases := []struct {
		name  string
		value interface{}
		want  int
	}{
		{"bool true", true, 1},
		{"bool false", false, 0},
		{"float 1", float64(1), 1},
		{"float 0", float64(0), 0},
		{"json number 1", json.Number("1"), 1},
		{"json number 0", json.Number("0"), 0},
		{"json number float", json.Number("1.5"), 1},
		{"int 1", 1, 1},
		{"int32 0", int32(0), 0},
		{"int64 7", int64(7), 1},
		{"string 1", "1", 1},
		{"string true", "TRUE", 1},
		{"string false", "false", 0},
		{"unknown", struct{}{}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := models.PrivacyValueToInt(tc.value); got != tc.want {
				t.Fatalf("got %d want %d", got, tc.want)
			}
		})
	}
}

func TestPrivacyIntToBoolMap(t *testing.T) {
	got := models.PrivacyIntToBoolMap(map[string]int{"name": 1, "email": 0})
	if !got["name"] || got["email"] {
		t.Fatalf("got=%v", got)
	}
	defaults := models.PrivacyIntToBoolMap(nil)
	if !defaults["name"] || defaults["phone"] {
		t.Fatalf("nil input should use defaults, got name=%v phone=%v", defaults["name"], defaults["phone"])
	}
}

func TestPrivacyIntToInt32Map(t *testing.T) {
	got := models.PrivacyIntToInt32Map(map[string]int{"score": 1, "phone": 0})
	if got["score"] != 1 || got["phone"] != 0 {
		t.Fatalf("got=%v", got)
	}
	defaults := models.PrivacyIntToInt32Map(nil)
	if defaults["name"] != 1 {
		t.Fatalf("nil input should use defaults, got name=%d", defaults["name"])
	}
}

func TestParsePublicPrivacyJSON(t *testing.T) {
	t.Run("missing keys default to public", func(t *testing.T) {
		got := models.ParsePublicPrivacyJSON(`{"name":1,"email":0}`)
		if got["phone"] != 1 || got["occupation"] != 1 {
			t.Fatalf("missing keys must be public, got phone=%d occupation=%d", got["phone"], got["occupation"])
		}
		if got["email"] != 0 {
			t.Fatalf("explicit private key must remain private, got email=%d", got["email"])
		}
	})
}
