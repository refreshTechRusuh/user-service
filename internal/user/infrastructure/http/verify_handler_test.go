package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

type fakeVerifier struct {
	res   dto.VerifyNIKResponse
	err   error
	calls int
}

func (f *fakeVerifier) VerifyNIK(context.Context, dto.VerifyNIKRequest) (dto.VerifyNIKResponse, error) {
	f.calls++
	return f.res, f.err
}

// fakeLimiter menghitung per key di memori; failKeyPrefix memaksa error Redis.
type fakeLimiter struct {
	counts map[string]int
	fail   bool
}

func (f *fakeLimiter) Allow(_ context.Context, key string, limit int, _ time.Duration) (bool, error) {
	if f.fail {
		return false, errors.New("redis down")
	}
	if f.counts == nil {
		f.counts = map[string]int{}
	}
	f.counts[key]++
	return f.counts[key] <= limit, nil
}

const goodBody = `{"nik":"3273000000000001","nama":"Budi","tglLahir":"1990-01-01","namaIbu":"Siti"}`

func post(h *VerifyHandler, body, ip string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/user/v1/nik/verify", strings.NewReader(body))
	req.Header.Set("X-Forwarded-For", ip)
	rr := httptest.NewRecorder()
	h.Verify(rr, req)
	return rr
}

func TestVerify_ShouldReturn200Valid_WhenDukcapilMatches(t *testing.T) {
	v := &fakeVerifier{res: dto.VerifyNIKResponse{Valid: true}}
	h := NewVerifyHandler(v, &fakeLimiter{}, 10, 5)

	rr := post(h, goodBody, "1.1.1.1")

	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"valid":true`) {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestVerify_ShouldReturn200ValidFalse_WhenNotMatching(t *testing.T) {
	h := NewVerifyHandler(&fakeVerifier{res: dto.VerifyNIKResponse{Valid: false}}, &fakeLimiter{}, 10, 5)

	rr := post(h, goodBody, "1.1.1.1")

	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"valid":false`) {
		t.Fatalf("code=%d body=%s", rr.Code, rr.Body.String())
	}
}

func TestVerify_ShouldReturn400_WhenBodyInvalid(t *testing.T) {
	v := &fakeVerifier{}
	h := NewVerifyHandler(v, &fakeLimiter{}, 10, 5)

	for name, body := range map[string]string{
		"bukan json":      `{`,
		"nik bukan angka": `{"nik":"abc","nama":"B","tglLahir":"1990-01-01","namaIbu":"S"}`,
		"field kosong":    `{"nik":"3273000000000001"}`,
	} {
		if rr := post(h, body, "1.1.1.1"); rr.Code != http.StatusBadRequest {
			t.Errorf("%s: code=%d; mau 400", name, rr.Code)
		}
	}
	if v.calls != 0 {
		t.Errorf("usecase terpanggil %d kali; request invalid tak boleh sampai Dukcapil", v.calls)
	}
}

func TestVerify_ShouldReturn400_WhenBodyTooLarge(t *testing.T) {
	h := NewVerifyHandler(&fakeVerifier{}, &fakeLimiter{}, 10, 5)
	big := `{"nik":"3273000000000001","nama":"` + strings.Repeat("a", 5000) + `","tglLahir":"x","namaIbu":"y"}`

	if rr := post(h, big, "1.1.1.1"); rr.Code != http.StatusBadRequest {
		t.Fatalf("code=%d; mau 400 untuk body > 4KB", rr.Code)
	}
}

func TestVerify_ShouldReturn503_WhenDukcapilUnavailable(t *testing.T) {
	v := &fakeVerifier{err: apperror.DukcapilUnavailable(errors.New("timeout"))}
	h := NewVerifyHandler(v, &fakeLimiter{}, 10, 5)

	if rr := post(h, goodBody, "1.1.1.1"); rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d; mau 503", rr.Code)
	}
}

func TestVerify_ShouldReturn429_WhenIPExceedsLimit(t *testing.T) {
	h := NewVerifyHandler(&fakeVerifier{}, &fakeLimiter{}, 2, 100)

	post(h, goodBody, "9.9.9.9")
	post(h, goodBody, "9.9.9.9")
	rr := post(h, goodBody, "9.9.9.9")

	if rr.Code != http.StatusTooManyRequests || rr.Header().Get("Retry-After") == "" {
		t.Fatalf("code=%d retry-after=%q; mau 429 + Retry-After", rr.Code, rr.Header().Get("Retry-After"))
	}
	if other := post(h, goodBody, "8.8.8.8"); other.Code != http.StatusOK {
		t.Errorf("IP lain code=%d; limit harus per IP", other.Code)
	}
}

func TestVerify_ShouldReturn429_WhenSameNIKFromManyIPs(t *testing.T) {
	v := &fakeVerifier{}
	h := NewVerifyHandler(v, &fakeLimiter{}, 100, 2)

	post(h, goodBody, "1.1.1.1")
	post(h, goodBody, "2.2.2.2")
	rr := post(h, goodBody, "3.3.3.3") // IP berganti-ganti, NIK sama

	if rr.Code != http.StatusTooManyRequests {
		t.Fatalf("code=%d; mau 429 (limit per NIK tak boleh bisa dilewati ganti IP)", rr.Code)
	}
	if v.calls != 2 {
		t.Errorf("Dukcapil terpanggil %d kali; mau 2", v.calls)
	}
}

func TestVerify_ShouldFailOpen_WhenRedisDown(t *testing.T) {
	h := NewVerifyHandler(&fakeVerifier{res: dto.VerifyNIKResponse{Valid: true}}, &fakeLimiter{fail: true}, 1, 1)

	if rr := post(h, goodBody, "1.1.1.1"); rr.Code != http.StatusOK {
		t.Fatalf("code=%d; limiter error tak boleh memutus journey", rr.Code)
	}
}

func TestVerify_ShouldSkipLimit_WhenLimitIsZero(t *testing.T) {
	h := NewVerifyHandler(&fakeVerifier{}, &fakeLimiter{}, 0, 0)

	for i := 0; i < 5; i++ {
		if rr := post(h, goodBody, "1.1.1.1"); rr.Code != http.StatusOK {
			t.Fatalf("request %d code=%d; limit 0 = mati", i, rr.Code)
		}
	}
}
