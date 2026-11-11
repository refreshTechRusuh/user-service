package apperror

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConstructors_ShouldSetCodeAndMessage(t *testing.T) {
	cause := errors.New("db down")
	cases := []struct {
		name string
		err  *AppError
		code ErrorCode
	}{
		{"Validation", Validation("x"), ErrValidation},
		{"Unauthorized", Unauthorized("x"), ErrUnauthorized},
		{"InvalidCredentials", InvalidCredentials(), ErrInvalidCredentials},
		{"Forbidden", Forbidden(), ErrForbidden},
		{"NotFound", NotFound("member"), ErrNotFound},
		{"EmailTaken", EmailTaken(), ErrEmailTaken},
		{"NIKInvalid", NIKInvalid(), ErrNIKInvalid},
		{"MemberDuplicate", MemberDuplicate(), ErrMemberDuplicate},
		{"RateLimited", RateLimited(), ErrRateLimited},
		{"DukcapilUnavailable", DukcapilUnavailable(cause), ErrDukcapilUnavailable},
		{"Internal", Internal(cause), ErrInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.code, tc.err.Code)
			assert.NotEmpty(t, tc.err.Message)
		})
	}
}

func TestNotFound_ShouldIncludeResourceName(t *testing.T) {
	assert.Equal(t, "member tidak ditemukan", NotFound("member").Message)
}

func TestError_ShouldIncludeCause_WhenWrapped(t *testing.T) {
	cause := errors.New("boom")

	wrapped := Wrap(ErrInternal, "gagal", cause)

	assert.Contains(t, wrapped.Error(), "boom")
	assert.Contains(t, wrapped.Error(), string(ErrInternal))
	assert.ErrorIs(t, wrapped, cause, "Unwrap harus mengembalikan penyebab asli")
}

func TestError_ShouldOmitCause_WhenNotWrapped(t *testing.T) {
	err := New(ErrValidation, "salah")

	assert.Equal(t, "[VALIDATION_ERROR] salah", err.Error())
	assert.NoError(t, errors.Unwrap(err))
}

func TestInternal_ShouldNotExposeCauseInMessage(t *testing.T) {
	err := Internal(errors.New("password=rahasia"))

	require.NotContains(t, err.Message, "rahasia", "pesan ke klien tidak boleh membocorkan penyebab internal")
}
