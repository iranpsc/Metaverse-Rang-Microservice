package models_test

import (
	"testing"
	"time"

	"metarang/buildings-service/internal/models"
)

func TestNullableDateTimeScanAndValue(t *testing.T) {
	now := time.Date(2024, 6, 1, 12, 30, 0, 0, time.UTC)
	cases := []struct {
		name  string
		input any
		valid bool
	}{
		{"nil", nil, false},
		{"time", now, true},
		{"bytes", []byte("2024-06-01 12:30:00"), true},
		{"string", "2024-06-01T12:30:00Z", true},
		{"empty", " ", false},
		{"zero", "0000-00-00 00:00:00", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var value models.NullableDateTime
			if err := value.Scan(tc.input); err != nil {
				t.Fatal(err)
			}
			if value.Valid != tc.valid {
				t.Fatalf("valid=%v want %v", value.Valid, tc.valid)
			}
			stored, err := value.Value()
			if err != nil {
				t.Fatal(err)
			}
			if tc.valid && stored == nil || !tc.valid && stored != nil {
				t.Fatalf("stored=%v", stored)
			}
		})
	}
	var value models.NullableDateTime
	if err := value.Scan(42); err == nil {
		t.Fatal("unsupported value accepted")
	}
	if err := value.Scan("not a date"); err == nil {
		t.Fatal("invalid date accepted")
	}
	var nilValue *models.NullableDateTime
	if err := nilValue.Scan(now); err == nil {
		t.Fatal("nil receiver accepted")
	}
}
