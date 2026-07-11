package handler_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"metargb/features-service/internal/handler"
)

func TestSetProjectLocale_Normalize(t *testing.T) {
	handler.SetProjectLocale("  FA  ")
	assert.Equal(t, "fa", handler.GetProjectLocale())
	handler.SetProjectLocale("invalid")
	assert.Equal(t, "en", handler.GetProjectLocale())
	handler.SetProjectLocale("EN")
	assert.Equal(t, "en", handler.GetProjectLocale())
}

func TestGetProjectLocale_Default(t *testing.T) {
	handler.SetProjectLocale("")
	assert.Equal(t, "en", handler.GetProjectLocale())
}
