package middleware

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeLimiter menghitung per key di memori.
type fakeLimiter struct {
	counts map[string]int
	keys   []string
	err    error
}

func (f *fakeLimiter) Allow(_ context.Context, key string, limit int, _ time.Duration) (bool, error) {
	f.keys = append(f.keys, key)
	if f.err != nil {
		return false, f.err
	}
	if f.counts == nil {
		f.counts = map[string]int{}
	}
	f.counts[key]++
	return f.counts[key] <= limit, nil
}

func hit(h http.Handler, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.Header.Set("X-Forwarded-For", ip)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func counting(called *int) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { *called++ })
}

func TestRateLimit_ShouldBlockAfterLimit_With429AndRetryAfter(t *testing.T) {
	var calls int
	h := RateLimit(&fakeLimiter{}, "login", 2, time.Minute, ClientIPKey)(counting(&calls))

	hit(h, "1.1.1.1")
	hit(h, "1.1.1.1")
	rr := hit(h, "1.1.1.1")

	assert.Equal(t, http.StatusTooManyRequests, rr.Code)
	assert.Equal(t, "60", rr.Header().Get("Retry-After"))
	assert.Equal(t, "application/json", rr.Header().Get("Content-Type"))
	assert.Contains(t, rr.Body.String(), `"status":"429"`)
	assert.Equal(t, 2, calls, "request yang diblokir tidak boleh sampai ke handler")
}

func TestRateLimit_ShouldCountPerKey(t *testing.T) {
	var calls int
	h := RateLimit(&fakeLimiter{}, "login", 1, time.Minute, ClientIPKey)(counting(&calls))

	a1 := hit(h, "1.1.1.1")
	a2 := hit(h, "1.1.1.1")
	b1 := hit(h, "2.2.2.2")

	assert.Equal(t, http.StatusOK, a1.Code)
	assert.Equal(t, http.StatusTooManyRequests, a2.Code)
	assert.Equal(t, http.StatusOK, b1.Code, "IP lain tidak terpengaruh")
}

func TestRateLimit_ShouldPrefixKeyWithName_SoLimitersDoNotShareCounters(t *testing.T) {
	l := &fakeLimiter{}
	var calls int
	login := RateLimit(l, "login:ip", 1, time.Minute, ClientIPKey)(counting(&calls))
	register := RateLimit(l, "register:ip", 1, time.Minute, ClientIPKey)(counting(&calls))

	hit(login, "1.1.1.1")
	rr := hit(register, "1.1.1.1")

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, []string{"login:ip:1.1.1.1", "register:ip:1.1.1.1"}, l.keys)
}

func TestRateLimit_ShouldPassThrough_WhenDisabledOrNoKey(t *testing.T) {
	t.Run("limit 0 → mati", func(t *testing.T) {
		var calls int
		l := &fakeLimiter{}
		h := RateLimit(l, "x", 0, time.Minute, ClientIPKey)(counting(&calls))
		for i := 0; i < 5; i++ {
			hit(h, "1.1.1.1")
		}
		assert.Equal(t, 5, calls)
		assert.Empty(t, l.keys, "limiter tidak dipanggil sama sekali")
	})
	t.Run("limiter nil → lolos", func(t *testing.T) {
		var calls int
		h := RateLimit(nil, "x", 1, time.Minute, ClientIPKey)(counting(&calls))
		hit(h, "1.1.1.1")
		hit(h, "1.1.1.1")
		assert.Equal(t, 2, calls)
	})
	t.Run("keyFn kosong → tidak dibatasi", func(t *testing.T) {
		var calls int
		l := &fakeLimiter{}
		h := RateLimit(l, "x", 1, time.Minute, func(*http.Request) string { return "" })(counting(&calls))
		hit(h, "1.1.1.1")
		hit(h, "1.1.1.1")
		assert.Equal(t, 2, calls)
		assert.Empty(t, l.keys)
	})
}

func TestRateLimit_ShouldFailOpen_WhenLimiterErrors(t *testing.T) {
	var calls int
	h := RateLimit(&fakeLimiter{err: errors.New("redis down")}, "x", 1, time.Minute, ClientIPKey)(counting(&calls))

	rr := hit(h, "1.1.1.1")

	assert.Equal(t, http.StatusOK, rr.Code, "Redis mati tidak boleh memutus login")
	assert.Equal(t, 1, calls)
}

func TestClientIPKey(t *testing.T) {
	t.Run("X-Forwarded-For: nilai pertama", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Forwarded-For", "1.1.1.1, 2.2.2.2")
		assert.Equal(t, "1.1.1.1", ClientIPKey(r))
	})
	t.Run("tanpa XFF → RemoteAddr tanpa port", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "3.3.3.3:5555"
		assert.Equal(t, "3.3.3.3", ClientIPKey(r))
	})
	t.Run("RemoteAddr tanpa port → apa adanya", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.RemoteAddr = "4.4.4.4"
		assert.Equal(t, "4.4.4.4", ClientIPKey(r))
	})
	t.Run("XFF hanya spasi/koma → jatuh ke RemoteAddr", func(t *testing.T) {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		r.Header.Set("X-Forwarded-For", " , ")
		r.RemoteAddr = "5.5.5.5:1"
		assert.Equal(t, "5.5.5.5", ClientIPKey(r))
	})
}

func jsonReq(body string) *http.Request {
	return httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body))
}

func TestBodyFieldKey_ShouldHashAndNormaliseValue(t *testing.T) {
	key := BodyFieldKey("email")

	a := key(jsonReq(`{"email":"Budi@Mail.com","password":"x"}`))
	b := key(jsonReq(`{"email":"  budi@mail.com ","password":"y"}`))
	c := key(jsonReq(`{"email":"lain@mail.com"}`))

	require.NotEmpty(t, a)
	assert.Equal(t, a, b, "beda huruf besar/spasi → akun yang sama → key sama")
	assert.NotEqual(t, a, c)
	assert.NotContains(t, a, "budi", "email mentah tidak boleh jadi key Redis")
	assert.Len(t, a, 32)
}

func TestBodyFieldKey_ShouldReturnEmpty_WhenNotUsable(t *testing.T) {
	key := BodyFieldKey("email")
	for name, body := range map[string]string{
		"bukan json":      `bukan json`,
		"field tak ada":   `{"password":"x"}`,
		"field bukan str": `{"email":123}`,
		"field kosong":    `{"email":"  "}`,
		"kosong":          ``,
	} {
		assert.Empty(t, key(jsonReq(body)), name)
	}
	r := jsonReq(`{}`)
	r.Body = nil
	assert.Empty(t, key(r), "tanpa body")
}

func TestBodyFieldKey_ShouldRestoreBodyForDownstreamHandler(t *testing.T) {
	key := BodyFieldKey("email")
	original := `{"email":"budi@mail.com","password":"rahasia123"}`
	r := jsonReq(original)

	_ = key(r)
	rest, err := io.ReadAll(r.Body)

	require.NoError(t, err)
	assert.Equal(t, original, string(rest), "handler harus tetap bisa membaca body utuh")
}

func TestBodyFieldKey_ShouldRestoreWholeBody_EvenWhenLargerThanPeekLimit(t *testing.T) {
	key := BodyFieldKey("email")
	pad := strings.Repeat("a", peekLimit*2)
	original := `{"email":"budi@mail.com","pad":"` + pad + `"}`
	r := jsonReq(original)

	_ = key(r)
	rest, err := io.ReadAll(r.Body)

	require.NoError(t, err)
	assert.Equal(t, len(original), len(rest), "bagian di luar batas intip tidak boleh hilang")
	assert.Equal(t, original, string(rest))
}

func TestRateLimit_WithBodyFieldKey_ShouldLimitPerAccountAcrossIPs(t *testing.T) {
	var seen string
	h := RateLimit(&fakeLimiter{}, "login:email", 2, time.Minute, BodyFieldKey("email"))(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			seen = string(b)
		}))
	post := func(ip, email string) int {
		req := jsonReq(`{"email":"` + email + `","password":"x"}`)
		req.Header.Set("X-Forwarded-For", ip)
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr.Code
	}

	post("1.1.1.1", "korban@mail.com")
	post("2.2.2.2", "korban@mail.com")
	third := post("3.3.3.3", "korban@mail.com") // IP berganti-ganti, akun sama
	other := post("3.3.3.3", "lain@mail.com")

	assert.Equal(t, http.StatusTooManyRequests, third, "penyerang tak bisa lolos dengan memalsukan IP")
	assert.Equal(t, http.StatusOK, other)
	assert.Contains(t, seen, "lain@mail.com", "handler menerima body utuh setelah diintip")
}
