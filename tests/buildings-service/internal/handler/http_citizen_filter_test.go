package handler_test

import (
	"reflect"
	"testing"

	"metarang/buildings-service/internal/handler"
)

func TestFilterAllowedKarbaris(t *testing.T) {
	t.Run("defaults to displayable types", func(t *testing.T) {
		want := []string{"a", "m", "t", "g", "s", "b", "e", "n"}
		if got := handler.FilterAllowedKarbaris(nil, nil); !reflect.DeepEqual(got, want) {
			t.Fatalf("FilterAllowedKarbaris(nil, nil) = %v, want %v", got, want)
		}
	})

	t.Run("respects privacy settings", func(t *testing.T) {
		privacy := map[string]int32{"tejari_features": 0, "maskoni_features": 1}
		want := []string{"m", "a"}
		if got := handler.FilterAllowedKarbaris(privacy, []string{"t", "m", "a"}); !reflect.DeepEqual(got, want) {
			t.Fatalf("FilterAllowedKarbaris() = %v, want %v", got, want)
		}
	})

	t.Run("rejects unmapped codes", func(t *testing.T) {
		want := []string{"t", "m"}
		if got := handler.FilterAllowedKarbaris(map[string]int32{}, []string{"t", "f", "p", "z", "unknown", "m"}); !reflect.DeepEqual(got, want) {
			t.Fatalf("FilterAllowedKarbaris() = %v, want %v", got, want)
		}
	})

	t.Run("missing privacy key defaults visible", func(t *testing.T) {
		want := []string{"t"}
		if got := handler.FilterAllowedKarbaris(map[string]int32{}, []string{"t"}); !reflect.DeepEqual(got, want) {
			t.Fatalf("FilterAllowedKarbaris() = %v, want %v", got, want)
		}
	})
}
