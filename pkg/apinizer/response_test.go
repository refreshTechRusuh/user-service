package apinizer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

func decode(t *testing.T, rr *httptest.ResponseRecorder) Envelope {
	t.Helper()
	var env Envelope
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &env))
	return env
}

func TestSuccess_ShouldWriteEnvelope200(t *testing.T) {
	rr := httptest.NewRecorder()

	Success(rr, "OK", map[string]bool{"ok": true})

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	env := decode(t, rr)
	assert.Equal(t, "00000", env.Status)
	assert.Equal(t, "OK", env.Message)
}

func TestCreated_ShouldWriteEnvelope201(t *testing.T) {
	rr := httptest.NewRecorder()

	Created(rr, "dibuat", nil)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.Equal(t, "00000", decode(t, rr).Status)
}

func TestFailedBadRequest_ShouldWrite400(t *testing.T) {
	rr := httptest.NewRecorder()

	FailedBadRequest(rr, "body tidak valid")

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	env := decode(t, rr)
	assert.Equal(t, "400", env.Status)
	assert.Equal(t, "body tidak valid", env.Message)
}

func TestWriteError_ShouldMapErrorCodeToHTTPStatus(t *testing.T) {
	cases := []struct {
		err        *apperror.AppError
		wantStatus int
	}{
		{apperror.Validation("x"), http.StatusBadRequest},
		{apperror.NIKInvalid(), http.StatusBadRequest},
		{apperror.Unauthorized("x"), http.StatusUnauthorized},
		{apperror.InvalidCredentials(), http.StatusUnauthorized},
		{apperror.New(apperror.ErrTokenInvalid, "x"), http.StatusUnauthorized},
		{apperror.New(apperror.ErrTokenExpired, "x"), http.StatusUnauthorized},
		{apperror.Forbidden(), http.StatusForbidden},
		{apperror.NotFound("member"), http.StatusNotFound},
		{apperror.EmailTaken(), http.StatusConflict},
		{apperror.MemberDuplicate(), http.StatusConflict},
		{apperror.RateLimited(), http.StatusTooManyRequests},
		{apperror.DukcapilUnavailable(errors.New("x")), http.StatusServiceUnavailable},
		{apperror.Internal(errors.New("x")), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		t.Run(string(tc.err.Code), func(t *testing.T) {
			rr := httptest.NewRecorder()

			WriteError(rr, tc.err)

			assert.Equal(t, tc.wantStatus, rr.Code)
			assert.Equal(t, fmt.Sprint(tc.wantStatus), decode(t, rr).Status)
		})
	}
}

func TestWriteError_ShouldHideDetails_WhenErrorIsNotAppError(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteError(rr, errors.New("connection refused to 10.0.0.5"))

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.NotContains(t, rr.Body.String(), "10.0.0.5", "detail internal tidak boleh bocor")
}

func TestWriteError_ShouldUnwrapWrappedAppError(t *testing.T) {
	rr := httptest.NewRecorder()

	WriteError(rr, fmt.Errorf("konteks: %w", apperror.EmailTaken()))

	assert.Equal(t, http.StatusConflict, rr.Code)
}
