package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidateRequired_uint64Zero(t *testing.T) {
	errs := validateRequired("fid", uint64(0), "en")
	require.Contains(t, errs, "fid")
}

func TestValidateRequired_stringEmpty(t *testing.T) {
	errs := validateRequired("name", "", "en")
	require.Contains(t, errs, "name")
}

func TestValidateOneOf(t *testing.T) {
	errs := validateOneOf("k", "x", []string{"m", "t", "a"}, "en")
	require.Contains(t, errs, "k")
	errsOK := validateOneOf("k", "m", []string{"m", "t", "a"}, "en")
	assert.Empty(t, errsOK)
}

func TestMergeValidationErrors(t *testing.T) {
	m := mergeValidationErrors(
		map[string]string{"a": "1"},
		map[string]string{"b": "2"},
	)
	assert.Len(t, m, 2)
}

func TestReturnValidationError_GRPCCode(t *testing.T) {
	err := returnValidationError(map[string]string{"f": "msg"})
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestValidateMin(t *testing.T) {
	errs := validateMin("n", 1, 5, "en")
	require.Contains(t, errs, "n")
	assert.Empty(t, validateMin("n", 10, 5, "en"))
}

func TestValidateMinLength(t *testing.T) {
	errs := validateMinLength("p", "ab", 5, "en")
	require.Contains(t, errs, "p")
}
