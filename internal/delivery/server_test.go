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
	return newHandlerWith(t, dukcapil, nil)
}

// newHandlerWith sama seperti newHandler, dengan kesempatan mengubah config (mis. batas rate limit).
func newHandlerWith(t *testing.T, dukcapil http.HandlerFunc, mutate func(*config.Config)) http.Handler {
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
	if mutate != nil {
		mutate(cfg)
	}

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

// ---------------------------------------------------------------- keamanan

var noop = func(http.ResponseWriter, *http.Request) {}

func TestNew_ShouldSetSecurityHeaders_OnEveryKindOfResponse(t *testing.T) {
	h := newHandler(t, noop)

	for name, rr := range map[string]*httptest.ResponseRecorder{
		"200 health":        serve(h, http.MethodGet, "/sys/ping", ""),
		"404 route tak ada": serve(h, http.MethodGet, "/tidak/ada", ""),
		"401 butuh login":   serve(h, http.MethodGet, "/auth/me", ""),
		"400 validasi":      serve(h, http.MethodPost, "/auth/login", `{}`),
		"preflight OPTIONS": serve(h, http.MethodOptions, "/auth/login", ""),
	} {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
			assert.Equal(t, "default-src 'none'", rr.Header().Get("Content-Security-Policy"))
			assert.Equal(t, "no-store", rr.Header().Get("Cache-Control"))
			assert.NotEmpty(t, rr.Header().Get("Strict-Transport-Security"))
			assert.Empty(t, rr.Header().Get("X-Frame-Options"))
		})
	}
}

func TestNew_ShouldReturn413_WithCORSAndSecurityHeaders_WhenBodyTooLarge(t *testing.T) {
	h := newHandlerWith(t, noop, func(c *config.Config) { c.Security.MaxBodyBytes = 64 })
	req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(strings.Repeat("a", 200)))
	req.Header.Set("Origin", "https://fe.example")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rr.Code)
	assert.Equal(t, "https://fe.example", rr.Header().Get("Access-Control-Allow-Origin"),
		"413 harus membawa header CORS, kalau tidak browser melaporkan CORS error")
	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
}

func TestNew_ShouldRateLimitLogin_PerIP(t *testing.T) {
	h := newHandlerWith(t, noop, func(c *config.Config) {
		c.Security.LoginPerIPPerMin = 3
		c.Security.LoginPerEmailPerMin = 0 // matikan lapis email agar yang diuji hanya IP
	})
	login := func(ip string) int {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"a@b.com","password":"x"}`))
		req.Header.Set("X-Forwarded-For", ip)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	var last int
	for i := 0; i < 4; i++ {
		last = login("9.9.9.9")
	}

	assert.Equal(t, http.StatusTooManyRequests, last)
	assert.NotEqual(t, http.StatusTooManyRequests, login("8.8.8.8"), "IP lain tidak terpengaruh")
}

func TestNew_ShouldRateLimitLogin_PerEmail_EvenWhenIPKeepsChanging(t *testing.T) {
	h := newHandlerWith(t, noop, func(c *config.Config) {
		c.Security.LoginPerIPPerMin = 0 // IP tak dibatasi: penyerang memalsukan X-Forwarded-For
		c.Security.LoginPerEmailPerMin = 3
	})
	login := func(ip, email string) int {
		req := httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"email":"`+email+`","password":"tebakan"}`))
		req.Header.Set("X-Forwarded-For", ip)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	codes := []int{
		login("1.1.1.1", "korban@mail.com"),
		login("2.2.2.2", "korban@mail.com"),
		login("3.3.3.3", "Korban@Mail.com"), // beda huruf besar = akun yang sama
		login("4.4.4.4", "korban@mail.com"),
	}

	assert.Equal(t, http.StatusTooManyRequests, codes[3], "tebakan password dibatasi per akun walau IP berganti-ganti")
	assert.NotEqual(t, http.StatusTooManyRequests, login("5.5.5.5", "lain@mail.com"))
}

func TestNew_ShouldRateLimitRegister_PerIP(t *testing.T) {
	h := newHandlerWith(t, noop, func(c *config.Config) { c.Security.RegisterPerIPPerMin = 2 })
	register := func() int {
		req := httptest.NewRequest(http.MethodPost, "/auth/register", strings.NewReader(`{}`))
		req.Header.Set("X-Forwarded-For", "7.7.7.7")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	register()
	register()

	assert.Equal(t, http.StatusTooManyRequests, register())
}

func TestNew_ShouldNotRateLimit_WhenLimitsAreZero(t *testing.T) {
	h := newHandlerWith(t, noop, func(c *config.Config) {
		c.Security.LoginPerIPPerMin, c.Security.LoginPerEmailPerMin, c.Security.RegisterPerIPPerMin = 0, 0, 0
	})

	for i := 0; i < 20; i++ {
		require.NotEqual(t, http.StatusTooManyRequests, serve(h, http.MethodPost, "/auth/login", `{"email":"a@b.com","password":"x"}`).Code)
	}
}

func TestNew_Login_ShouldStillReachHandlerWithFullBody_AfterRateLimitPeek(t *testing.T) {
	h := newHandler(t, noop)

	// Body diintip middleware (untuk email). Handler masih harus melihat body utuh:
	// email valid + password kosong → validasi handler (400, bukan "JSON rusak").
	rr := serve(h, http.MethodPost, "/auth/login", `{"email":"a@b.com","password":""}`)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Contains(t, rr.Body.String(), "password wajib diisi")
}
