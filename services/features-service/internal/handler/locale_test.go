package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSetProjectLocale_Normalize(t *testing.T) {
	SetProjectLocale("  FA  ")
	assert.Equal(t, "fa", projectLocale)
	SetProjectLocale("invalid")
	assert.Equal(t, "en", projectLocale)
	SetProjectLocale("EN")
	assert.Equal(t, "en", projectLocale)
}

func TestGetProjectLocale_Default(t *testing.T) {
	projectLocale = ""
	assert.Equal(t, "en", getProjectLocale())
}
