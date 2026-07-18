package handler

import (
	"reflect"
	"testing"
)

func TestFilterAllowedKarbaris_DefaultsToDisplayable(t *testing.T) {
	got := filterAllowedKarbaris(nil, nil)
	if !reflect.DeepEqual(got, citizenDisplayableKarbaris) {
		t.Fatalf("got %v, want %v", got, citizenDisplayableKarbaris)
	}
}

func TestFilterAllowedKarbaris_RespectsPrivacy(t *testing.T) {
	privacy := map[string]int32{
		"tejari_features":  0,
		"maskoni_features": 1,
	}
	got := filterAllowedKarbaris(privacy, []string{"t", "m", "a"})
	want := []string{"m", "a"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFilterAllowedKarbaris_RejectsUnmappedCodes(t *testing.T) {
	got := filterAllowedKarbaris(map[string]int32{}, []string{"t", "f", "p", "z", "unknown", "m"})
	want := []string{"t", "m"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFilterAllowedKarbaris_MissingPrivacyKeyDefaultsVisible(t *testing.T) {
	got := filterAllowedKarbaris(map[string]int32{}, []string{"t"})
	want := []string{"t"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
