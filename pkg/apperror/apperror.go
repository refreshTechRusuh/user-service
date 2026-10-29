// Package apperror mendefinisikan tipe error domain user-service.
// Pola: usecase menaikkan AppError{Code, Message}; lapis HTTP (apinizer)
// memetakan Code -> HTTP status lewat errors.As. Logika tak kenal HTTP.
package apperror

import "fmt"

// ErrorCode adalah kode error domain yang dipetakan ke HTTP status di apinizer.
type ErrorCode string

const (
	// 400
	ErrValidation ErrorCode = "VALIDATION_ERROR"
	ErrNIKInvalid ErrorCode = "NIK_INVALID"

	// 401
	ErrUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrInvalidCredentials ErrorCode = "INVALID_CREDENTIALS"
	ErrTokenInvalid       ErrorCode = "TOKEN_INVALID"
	ErrTokenExpired       ErrorCode = "TOKEN_EXPIRED"

	// 403
	ErrForbidden ErrorCode = "FORBIDDEN"

	// 404
	ErrNotFound ErrorCode = "NOT_FOUND"

	// 409
	ErrEmailTaken      ErrorCode = "EMAIL_TAKEN"
	ErrMemberDuplicate ErrorCode = "MEMBER_DUPLICATE"

	// 500
	ErrInternal ErrorCode = "INTERNAL_ERROR"
)

// AppError adalah error domain dengan kode spesifik + pesan human-readable.
// Bungkus dengan errors.As di apinizer agar bisa di-unwrap ke AppError.
type AppError struct {
	Code    ErrorCode
	Message string
	cause   error
}

func New(code ErrorCode, message string) *AppError {
	return &AppError{Code: code, Message: message}
}

func Wrap(code ErrorCode, message string, cause error) *AppError {
	return &AppError{Code: code, Message: message, cause: cause}
}

func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.cause }

// ---- Constructor helpers -----------------------------------------------

func Validation(msg string) *AppError    { return New(ErrValidation, msg) }
func Unauthorized(msg string) *AppError  { return New(ErrUnauthorized, msg) }
func InvalidCredentials() *AppError      { return New(ErrInvalidCredentials, "Email atau password salah") }
func Forbidden() *AppError               { return New(ErrForbidden, "Akses ditolak") }
func NotFound(resource string) *AppError { return New(ErrNotFound, resource+" tidak ditemukan") }
func EmailTaken() *AppError              { return New(ErrEmailTaken, "Email sudah terdaftar") }
func NIKInvalid() *AppError {
	return New(ErrNIKInvalid, "NIK tidak valid: data tidak cocok dengan Dukcapil")
}
func MemberDuplicate() *AppError { return New(ErrMemberDuplicate, "NIK ini sudah tersimpan") }
func Internal(cause error) *AppError {
	return Wrap(ErrInternal, "Terjadi kesalahan pada server", cause)
}
