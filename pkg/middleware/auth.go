// Package middleware menyediakan middleware HTTP untuk user-service.
// Dipakai oleh service ini (mis. /auth/me) dan pola-nya dibagikan ke tim.
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"github.com/refreshTechRusuh/user-service/pkg/token"
)

// contextKey = tipe privat untuk key context (hindari tabrakan antar-package).
type contextKey string

const (
	ctxUserID contextKey = "userId"
	ctxRole   contextKey = "role"
	ctxSID    contextKey = "sessionId"
)

// SessionRevocation = denylist sesi (diimplementasikan token store di Redis).
// Boleh nil: pengecekan pencabutan dilewati (dipakai di test / service tanpa Redis).
type SessionRevocation interface {
	IsRevoked(ctx context.Context, id string) (bool, error)
}

// envelope JSON manual untuk balasan middleware (tak bisa pakai apinizer:
// apinizer import middleware? tidak — tapi kita sengaja minimal di sini).
const (
	bodyUnauthorized = `{"status":"401","message":"Token tidak valid atau tidak ada","data":{}}`
	bodyForbidden    = `{"status":"403","message":"Akses ditolak","data":{}}`
)

// RequireAuth MEWAJIBKAN authToken valid DAN sesinya belum dicabut (logout).
// Tanpa/invalid/dicabut → 401. Bila valid, userId, role & sessionId disimpan di context.
func RequireAuth(gen *token.Generator, rev SessionRevocation) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearer(r)
			if tokenStr == "" {
				writeJSON(w, http.StatusUnauthorized, bodyUnauthorized)
				return
			}
			claims, err := gen.VerifyAuthToken(tokenStr)
			if err != nil || sessionRevoked(r.Context(), rev, claims.SessionID) {
				writeJSON(w, http.StatusUnauthorized, bodyUnauthorized)
				return
			}
			ctx := withClaims(r.Context(), claims.Subject, claims.Role, claims.SessionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalAuth: bila ada authToken valid → isi context (userId, role).
// Bila tidak ada / tidak valid → LANJUT tanpa menolak (untuk guest journey).
// Dipakai Queue & dibagikan ke BE-2.
func OptionalAuth(gen *token.Generator, rev SessionRevocation) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if tokenStr := extractBearer(r); tokenStr != "" {
				if claims, err := gen.VerifyAuthToken(tokenStr); err == nil && !sessionRevoked(r.Context(), rev, claims.SessionID) {
					r = r.WithContext(withClaims(r.Context(), claims.Subject, claims.Role, claims.SessionID))
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole MEWAJIBKAN authToken valid DAN role tertentu (mis. "admin").
// Dibagikan ke BE-2 untuk endpoint admin Event.
func RequireRole(gen *token.Generator, rev SessionRevocation, role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := extractBearer(r)
			if tokenStr == "" {
				writeJSON(w, http.StatusUnauthorized, bodyUnauthorized)
				return
			}
			claims, err := gen.VerifyAuthToken(tokenStr)
			if err != nil || sessionRevoked(r.Context(), rev, claims.SessionID) {
				writeJSON(w, http.StatusUnauthorized, bodyUnauthorized)
				return
			}
			if claims.Role != role {
				writeJSON(w, http.StatusForbidden, bodyForbidden)
				return
			}
			ctx := withClaims(r.Context(), claims.Subject, claims.Role, claims.SessionID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// TransactionID menambahkan X-Transaction-Id ke response header (untuk tracing).
func TransactionID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		txID := r.Header.Get("X-Transaction-Id")
		if txID == "" {
			txID = uuid.NewString()
		}
		w.Header().Set("X-Transaction-Id", txID)
		next.ServeHTTP(w, r)
	})
}

// Chain menyusun middleware dari luar ke dalam (pertama = paling luar).
func Chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

// ---- helper baca context (dipakai handler) ----

// GetUserID mengambil userId dari context (di-set oleh RequireAuth/OptionalAuth).
func GetUserID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxUserID).(string)
	return v, ok
}

// GetRole mengambil role dari context.
func GetRole(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxRole).(string)
	return v, ok
}

// GetSessionID mengambil id sesi login dari context (kosong untuk token lama tanpa sid).
func GetSessionID(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ctxSID).(string)
	return v, ok
}

// ---- internal ----

func withClaims(ctx context.Context, userID, role, sessionID string) context.Context {
	ctx = context.WithValue(ctx, ctxUserID, userID)
	ctx = context.WithValue(ctx, ctxRole, role)
	ctx = context.WithValue(ctx, ctxSID, sessionID)
	return ctx
}

// sessionRevoked true bila sesi sudah dicabut (logout).
// rev nil atau sid kosong (token lama) → dianggap belum dicabut.
// Redis error → fail-open (log warning) agar gangguan Redis tidak mematikan semua
// endpoint ber-login; pola sama dengan rate limiter.
func sessionRevoked(ctx context.Context, rev SessionRevocation, sessionID string) bool {
	if rev == nil || sessionID == "" {
		return false
	}
	revoked, err := rev.IsRevoked(ctx, token.SessionRevokeID(sessionID))
	if err != nil {
		slog.WarnContext(ctx, "cek pencabutan sesi gagal, fail-open", "err", err)
		return false
	}
	return revoked
}

func extractBearer(r *http.Request) string {
	auth := r.Header.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}

func writeJSON(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(body))
}
