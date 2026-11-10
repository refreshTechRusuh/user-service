package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/refreshTechRusuh/user-service/internal/config"
)

// newHandler merakit service dengan Redis tiruan, Dukcapil tiruan, dan pool Postgres
// malas (pgxpool tidak konek sampai dipakai) — cukup untuk menguji rangkaian route.
func newHandler(t *testing.T, dukcapil http.HandlerFunc) http.Handler {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	pool, err := pgxpool.New(context.Background(), "postgres://u:p@127.0.0.1:1/db?connect_timeout=1")
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	srv := httptest.NewServer(dukcapil)
	t.Cleanup(srv.Close)

	cfg, err := config.Load()
	require.NoError(t, err)
	cfg.Dukcapil.BaseURL = srv.URL
	cfg.Admin = config.AdminBootstrapConfig{} // lewati bootstrap admin (butuh DB)

	h, err := New(context.Background(), cfg, pool, rdb)
	require.NoError(t, err)
	return h
}

func serve(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func TestNew_ShouldServeHealthCheck_WithTransactionID(t *testing.T) {
	h := newHandler(t, func(http.ResponseWriter, *http.Request) {})

	rr := serve(h, http.MethodGet, "/sys/ping", "")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "user-service ok")
	assert.NotEmpty(t, rr.Header().Get("X-Transaction-Id"))
}

func TestNew_ShouldProtectAuthenticatedRoutes(t *testing.T) {
	h := newHandler(t, func(http.ResponseWriter, *http.Request) {})

	for _, rt := range []struct{ method, path string }{
		{http.MethodGet, "/auth/me"},
		{http.MethodPost, "/api/user/v1/member/create"},
		{http.MethodGet, "/api/user/v1/member/list"},
		{http.MethodPut, "/api/user/v1/member/update/m1"},
		{http.MethodDelete, "/api/user/v1/member/delete/m1"},
		{http.MethodGet, "/api/user/v1/ticket-history"},
	} {
		t.Run(rt.method+" "+rt.path, func(t *testing.T) {
			assert.Equal(t, http.StatusUnauthorized, serve(h, rt.method, rt.path, "{}").Code)
		})
	}
}

func TestNew_ShouldExposeVerifyNIKPublicly_AndForwardToDukcapil(t *testing.T) {
	h := newHandler(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"00000","data":{"valid":true}}`))
	})

	rr := serve(h, http.MethodPost, "/api/user/v1/nik/verify",
		`{"nik":"3273000000000001","nama":"Budi","tglLahir":"1990-01-01","namaIbu":"Siti"}`)

	assert.Equal(t, http.StatusOK, rr.Code, "publik: tanpa token")
	assert.Contains(t, rr.Body.String(), `"valid":true`)
}

func TestNew_ShouldRateLimitVerifyNIK_PerNIK(t *testing.T) {
	h := newHandler(t, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":{"valid":false}}`))
	})
	body := `{"nik":"3273000000000009","nama":"B","tglLahir":"1990-01-01","namaIbu":"S"}`

	var last int
	for i := 0; i < 6; i++ { // batas default per NIK = 5/menit
		last = serve(h, http.MethodPost, "/api/user/v1/nik/verify", body).Code
	}

	assert.Equal(t, http.StatusTooManyRequests, last)
}

func TestNew_ShouldAllowPublicAuthEndpoints_ToReachHandlers(t *testing.T) {
	h := newHandler(t, func(http.ResponseWriter, *http.Request) {})

	// Body kosong → 400 dari handler (bukan 404/401) membuktikan route terpasang & publik.
	assert.Equal(t, http.StatusBadRequest, serve(h, http.MethodPost, "/auth/login", `{}`).Code)
	assert.Equal(t, http.StatusBadRequest, serve(h, http.MethodPost, "/auth/register", `{}`).Code)
	assert.Equal(t, http.StatusOK, serve(h, http.MethodPost, "/auth/logout", `{}`).Code)
}

func TestNew_ShouldReturn404_ForUnknownRoute(t *testing.T) {
	h := newHandler(t, func(http.ResponseWriter, *http.Request) {})

	assert.Equal(t, http.StatusNotFound, serve(h, http.MethodGet, "/tidak/ada", "").Code)
}
