// Package apinizer menyediakan envelope response standar:
//
//	{ status, message, data }  — status "00000" sukses, string kode HTTP gagal.
//
// Pola mengikuti event-service: Success/Created + WriteError (map apperror).
package apinizer

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

// Envelope adalah bentuk response tunggal untuk setiap endpoint.
type Envelope struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	Partial bool   `json:"partial,omitempty"`
}

// Success menulis response 200 OK dengan payload data.
func Success(w http.ResponseWriter, message string, data any) {
	writeJSON(w, http.StatusOK, Envelope{Status: "00000", Message: message, Data: data})
}

// Created menulis response 201 Created dengan payload data (mis. register).
func Created(w http.ResponseWriter, message string, data any) {
	writeJSON(w, http.StatusCreated, Envelope{Status: "00000", Message: message, Data: data})
}

// FailedBadRequest menulis response 400 (mis. body JSON tidak bisa di-parse).
func FailedBadRequest(w http.ResponseWriter, message string) {
	writeJSON(w, http.StatusBadRequest, Envelope{Status: "400", Message: message, Data: struct{}{}})
}

// WriteError memetakan apperror.AppError ke HTTP status + envelope gagal.
// Error tak terduga (bukan AppError) -> 500 dengan pesan generik.
func WriteError(w http.ResponseWriter, err error) {
	var appErr *apperror.AppError
	if !errors.As(err, &appErr) {
		slog.Error("unhandled error", "err", err)
		writeJSON(w, http.StatusInternalServerError, Envelope{
			Status:  "500",
			Message: "Terjadi kesalahan pada server",
			Data:    struct{}{},
		})
		return
	}

	status := mapErrorCode(appErr.Code)
	writeJSON(w, status, Envelope{Status: httpStatusStr(status), Message: appErr.Message, Data: struct{}{}})
}

// mapErrorCode memetakan ErrorCode ke HTTP status.
func mapErrorCode(code apperror.ErrorCode) int {
	switch code {
	case apperror.ErrValidation, apperror.ErrNIKInvalid:
		return http.StatusBadRequest
	case apperror.ErrUnauthorized, apperror.ErrInvalidCredentials,
		apperror.ErrTokenInvalid, apperror.ErrTokenExpired:
		return http.StatusUnauthorized
	case apperror.ErrForbidden:
		return http.StatusForbidden
	case apperror.ErrNotFound:
		return http.StatusNotFound
	case apperror.ErrEmailTaken, apperror.ErrMemberDuplicate:
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}

func httpStatusStr(status int) string {
	switch status {
	case 400:
		return "400"
	case 401:
		return "401"
	case 403:
		return "403"
	case 404:
		return "404"
	case 409:
		return "409"
	default:
		return "500"
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("apinizer: gagal encode response", "err", err)
	}
}
