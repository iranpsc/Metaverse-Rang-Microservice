package handler

import (
	"reflect"
	"testing"
)

func TestFilterAllowedKarbaris(t *testing.T) {
	t.Run("defaults to displayable types", func(t *testing.T) {
		want := []string{"a", "m", "t", "g", "s", "b", "e", "n"}
		if got := filterAllowedKarbaris(nil, nil); !reflect.DeepEqual(got, want) {
			t.Fatalf("filterAllowedKarbaris(nil, nil) = %v, want %v", got, want)
		}
	})

	t.Run("respects privacy settings", func(t *testing.T) {
		privacy := map[string]int32{"tejari_features": 0, "maskoni_features": 1}
		want := []string{"m", "a"}
		if got := filterAllowedKarbaris(privacy, []string{"t", "m", "a"}); !reflect.DeepEqual(got, want) {
			t.Fatalf("filterAllowedKarbaris() = %v, want %v", got, want)
		}
	})

	t.Run("rejects unmapped codes", func(t *testing.T) {
		want := []string{"t", "m"}
		if got := filterAllowedKarbaris(map[string]int32{}, []string{"t", "f", "p", "z", "unknown", "m"}); !reflect.DeepEqual(got, want) {
			t.Fatalf("filterAllowedKarbaris() = %v, want %v", got, want)
		}
	})

	t.Run("missing privacy key defaults visible", func(t *testing.T) {
		want := []string{"t"}
		if got := filterAllowedKarbaris(map[string]int32{}, []string{"t"}); !reflect.DeepEqual(got, want) {
			t.Fatalf("filterAllowedKarbaris() = %v, want %v", got, want)
		}
	})
}
