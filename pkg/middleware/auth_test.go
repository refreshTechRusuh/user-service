package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/refreshTechRusuh/user-service/pkg/token"
)

func newGen() *token.Generator {
	return token.NewGenerator("access-secret", "refresh-secret", time.Hour, 24*time.Hour)
}

// echo = handler akhir yang mengembalikan userId|role dari context.
var echo = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	uid, _ := GetUserID(r.Context())
	role, _ := GetRole(r.Context())
	_, _ = w.Write([]byte(uid + "|" + role))
})

func do(h http.Handler, authHeader string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func bearer(t *testing.T, g *token.Generator, uid, role string) string {
	t.Helper()
	tok, err := g.GenerateAuthToken(uid, role, "sid-1")
	require.NoError(t, err)
	return "Bearer " + tok
}

func TestRequireAuth(t *testing.T) {
	g := newGen()
	h := RequireAuth(g, nil)(echo)
	expired, err := token.NewGenerator("access-secret", "refresh-secret", -time.Minute, time.Hour).GenerateAuthToken("u1", "customer", "sid-1")
	require.NoError(t, err)
	wrongSecret, err := token.NewGenerator("lain", "refresh-secret", time.Hour, time.Hour).GenerateAuthToken("u1", "customer", "sid-1")
	require.NoError(t, err)

	t.Run("tanpa header → 401", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, do(h, "").Code)
	})
	t.Run("bukan Bearer → 401", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, do(h, "Basic abc").Code)
	})
	t.Run("token sampah → 401", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, do(h, "Bearer sampah").Code)
	})
	t.Run("token kedaluwarsa → 401", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, do(h, "Bearer "+expired).Code)
	})
	t.Run("signature secret lain → 401", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, do(h, "Bearer "+wrongSecret).Code)
	})
	t.Run("token sah → lanjut + context terisi", func(t *testing.T) {
		rr := do(h, bearer(t, g, "u1", "customer"))
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "u1|customer", rr.Body.String())
	})
	t.Run("body 401 berupa envelope JSON", func(t *testing.T) {
		rr := do(h, "")
		assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
		assert.True(t, strings.Contains(rr.Body.String(), `"status":"401"`))
	})
}

func TestOptionalAuth_ShouldAlwaysContinue(t *testing.T) {
	g := newGen()
	h := OptionalAuth(g, nil)(echo)

	t.Run("tanpa token → lanjut sebagai guest", func(t *testing.T) {
		rr := do(h, "")
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "|", rr.Body.String())
	})
	t.Run("token tidak valid → tetap lanjut tanpa identitas", func(t *testing.T) {
		rr := do(h, "Bearer sampah")
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "|", rr.Body.String())
	})
	t.Run("token sah → identitas terisi", func(t *testing.T) {
		rr := do(h, bearer(t, g, "u9", "customer"))
		assert.Equal(t, "u9|customer", rr.Body.String())
	})
}

func TestRequireRole(t *testing.T) {
	g := newGen()
	h := RequireRole(g, nil, "admin")(echo)

	t.Run("tanpa token → 401", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, do(h, "").Code)
	})
	t.Run("token tidak valid → 401", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, do(h, "Bearer sampah").Code)
	})
	t.Run("role customer → 403", func(t *testing.T) {
		assert.Equal(t, http.StatusForbidden, do(h, bearer(t, g, "u1", "customer")).Code)
	})
	t.Run("role admin → lanjut", func(t *testing.T) {
		rr := do(h, bearer(t, g, "a1", "admin"))
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "a1|admin", rr.Body.String())
	})
}

func TestTransactionID(t *testing.T) {
	h := TransactionID(echo)

	t.Run("dibuat bila tidak dikirim klien", func(t *testing.T) {
		rr := do(h, "")
		assert.NotEmpty(t, rr.Header().Get("X-Transaction-Id"))
	})
	t.Run("dipakai ulang bila dikirim klien", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Transaction-Id", "tx-123")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		assert.Equal(t, "tx-123", rr.Header().Get("X-Transaction-Id"))
	})
}

func TestChain_ShouldRunOutermostFirst(t *testing.T) {
	var order []string
	mk := func(name string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	do(Chain(echo, mk("luar"), mk("dalam")), "")

	assert.Equal(t, []string{"luar", "dalam"}, order)
}

func TestGetUserID_ShouldReportMissing_WhenContextEmpty(t *testing.T) {
	_, okU := GetUserID(httptest.NewRequest(http.MethodGet, "/", nil).Context())
	_, okR := GetRole(httptest.NewRequest(http.MethodGet, "/", nil).Context())

	assert.False(t, okU)
	assert.False(t, okR)
}

// stubRevocation = SessionRevocation dengan daftar id yang dicabut / error terkontrol.
type stubRevocation struct {
	revoked map[string]bool
	err     error
}

func (s stubRevocation) IsRevoked(_ context.Context, id string) (bool, error) {
	return s.revoked[id], s.err
}

func TestSessionRevocation_ShouldRejectRevokedSession_InAuthMiddlewares(t *testing.T) {
	g := newGen()
	rev := stubRevocation{revoked: map[string]bool{token.SessionRevokeID("sid-1"): true}}
	revoked := bearer(t, g, "u1", "admin") // sid-1 → dicabut
	other, err := g.GenerateAuthToken("u1", "admin", "sid-2")
	require.NoError(t, err)

	t.Run("RequireAuth: sesi dicabut → 401", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, do(RequireAuth(g, rev)(echo), revoked).Code)
	})
	t.Run("RequireAuth: sesi lain tetap lolos", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, do(RequireAuth(g, rev)(echo), "Bearer "+other).Code)
	})
	t.Run("RequireRole: sesi dicabut → 401", func(t *testing.T) {
		assert.Equal(t, http.StatusUnauthorized, do(RequireRole(g, rev, "admin")(echo), revoked).Code)
	})
	t.Run("OptionalAuth: sesi dicabut → lanjut sebagai guest", func(t *testing.T) {
		rr := do(OptionalAuth(g, rev)(echo), revoked)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, "|", rr.Body.String(), "context tidak boleh terisi")
	})
}

func TestSessionRevocation_ShouldFailOpen_WhenStoreErrors(t *testing.T) {
	g := newGen()
	rev := stubRevocation{err: errors.New("redis down")}
	assert.Equal(t, http.StatusOK, do(RequireAuth(g, rev)(echo), bearer(t, g, "u1", "customer")).Code)
}

func TestSessionRevocation_ShouldSkipCheck_WhenTokenHasNoSessionID(t *testing.T) {
	g := newGen()
	legacy, err := g.GenerateAuthToken("u1", "customer", "")
	require.NoError(t, err)
	rev := stubRevocation{revoked: map[string]bool{token.SessionRevokeID(""): true}}
	assert.Equal(t, http.StatusOK, do(RequireAuth(g, rev)(echo), "Bearer "+legacy).Code)
}
