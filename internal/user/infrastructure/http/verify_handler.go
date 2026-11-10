package http

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/refreshTechRusuh/user-service/internal/user/domain/dto"
	"github.com/refreshTechRusuh/user-service/internal/user/port/inbound"
	"github.com/refreshTechRusuh/user-service/internal/user/port/outbound"
	"github.com/refreshTechRusuh/user-service/pkg/apinizer"
	"github.com/refreshTechRusuh/user-service/pkg/apperror"
)

// maxVerifyBody = batas body endpoint publik ini. Isinya 4 string pendek.
const maxVerifyBody = 4 << 10

// VerifyHandler menangani POST /api/user/v1/nik/verify.
//
// Endpoint PUBLIK (guest boleh) yang menjawab "data ini cocok dengan Dukcapil atau tidak".
// Karena itu ia bisa disalahgunakan menebak data orang, dibatasi dua lapis:
//   - per IP   : meredam pemindaian massal (IP dari X-Forwarded-For bisa dipalsukan,
//     jadi ini bukan satu-satunya pagar);
//   - per NIK  : meredam penebakan nama/tgl lahir/nama ibu untuk satu NIK,
//     tidak peduli dari IP mana.
//
// Redis error -> fail-open: Dukcapil tetap di belakang, dan journey beli tiket tidak boleh
// putus karena limiter.
type VerifyHandler struct {
	uc         inbound.INIKVerifier
	limiter    outbound.IRateLimiter
	perIP      int // 0 = dimatikan
	perNIK     int // 0 = dimatikan
	rateWindow time.Duration
}

func NewVerifyHandler(uc inbound.INIKVerifier, limiter outbound.IRateLimiter, perIP, perNIK int) *VerifyHandler {
	return &VerifyHandler{uc: uc, limiter: limiter, perIP: perIP, perNIK: perNIK, rateWindow: time.Minute}
}

// Verify — POST /api/user/v1/nik/verify
func (h *VerifyHandler) Verify(w http.ResponseWriter, r *http.Request) {
	if !h.allow(w, r, "verifynik:ip:"+clientIP(r), h.perIP) {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxVerifyBody)
	var req dto.VerifyNIKRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apinizer.FailedBadRequest(w, "body tidak valid")
		return
	}
	if errs := req.Validate(); len(errs) > 0 {
		apinizer.FailedBadRequest(w, strings.Join(errs, "; "))
		return
	}

	if !h.allow(w, r, "verifynik:nik:"+hashKey(strings.TrimSpace(req.NIK)), h.perNIK) {
		return
	}

	res, err := h.uc.VerifyNIK(r.Context(), req)
	if err != nil {
		apinizer.WriteError(w, err)
		return
	}
	if res.Valid {
		apinizer.Success(w, "NIK valid", res)
		return
	}
	apinizer.Success(w, "Data tidak cocok dengan Dukcapil", res)
}

// allow mengembalikan false (dan sudah menulis 429) bila key melewati limit.
func (h *VerifyHandler) allow(w http.ResponseWriter, r *http.Request, key string, limit int) bool {
	if limit <= 0 || h.limiter == nil {
		return true
	}
	ok, err := h.limiter.Allow(r.Context(), key, limit, h.rateWindow)
	if err != nil {
		slog.WarnContext(r.Context(), "rate limit: redis error, fail-open", "err", err)
		return true
	}
	if !ok {
		w.Header().Set("Retry-After", "60")
		apinizer.WriteError(w, apperror.RateLimited())
		return false
	}
	return true
}

// hashKey menyamarkan nilai sensitif (NIK) sebelum dipakai sebagai key Redis.
func hashKey(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:16])
}

// clientIP: Cloud Run meneruskan IP asli di X-Forwarded-For (nilai pertama).
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip := strings.TrimSpace(strings.Split(xff, ",")[0]); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
