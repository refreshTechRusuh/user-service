package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Limiter = penghitung batas per key (fixed window). Harus berlaku lintas instance
// (Cloud Run bisa >1 instance) sehingga implementasinya memakai Redis.
type Limiter interface {
	// Allow mencatat satu aksi untuk key; false bila sudah melewati limit pada jendela ini.
	Allow(ctx context.Context, key string, limit int, window time.Duration) (bool, error)
}

const tooManyRequests = `{"status":"429","message":"Terlalu banyak permintaan, coba lagi sebentar","data":{}}`

// RateLimit membatasi request per key (hasil keyFn) per jendela waktu.
//
//   - limit <= 0, limiter nil, atau keyFn mengembalikan "" -> tidak dibatasi (lolos)
//   - Redis error -> fail-open: limiter tidak boleh memutus login/pembelian tiket
//   - melewati batas -> 429 + Retry-After
//
// name dipakai sebagai awalan key supaya beberapa limiter tidak saling menghitung.
func RateLimit(l Limiter, name string, limit int, window time.Duration, keyFn func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if limit <= 0 || l == nil {
			return next
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := keyFn(r)
			if id == "" {
				next.ServeHTTP(w, r)
				return
			}
			ok, err := l.Allow(r.Context(), name+":"+id, limit, window)
			if err != nil {
				slog.WarnContext(r.Context(), "rate limit: redis error, fail-open", "limiter", name, "err", err)
				next.ServeHTTP(w, r)
				return
			}
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", strconv.Itoa(int(window.Seconds())))
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(tooManyRequests))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ClientIPKey = key berdasarkan IP klien.
//
// Cloud Run meneruskan IP asli di X-Forwarded-For (nilai pertama). Header itu bisa
// dipalsukan klien yang memanggil langsung, jadi batas per-IP hanyalah satu lapis;
// lapis kedua yang tak bisa dielakkan adalah batas per-akun (lihat BodyFieldKey).
func ClientIPKey(r *http.Request) string {
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

// peekLimit = batas byte yang dibaca untuk mengintip field JSON.
const peekLimit = 8 << 10

// BodyFieldKey mengambil satu field string dari body JSON sebagai key (mis. "email"),
// lalu mengembalikan body utuh agar handler tetap bisa membacanya.
//
// Nilai di-lowercase, dipangkas, dan di-hash (sha256) sehingga data pribadi tidak
// tersimpan mentah sebagai key Redis. Body bukan JSON / field kosong -> "" (tidak dibatasi
// oleh limiter ini; handler tetap menolak body-nya).
func BodyFieldKey(field string) func(*http.Request) string {
	return func(r *http.Request) string {
		if r.Body == nil {
			return ""
		}
		head, _ := io.ReadAll(io.LimitReader(r.Body, peekLimit))
		// Kembalikan apa yang sudah dibaca + sisa body yang belum disentuh.
		r.Body = readCloser{Reader: io.MultiReader(bytes.NewReader(head), r.Body), Closer: r.Body}

		var m map[string]any
		if err := json.Unmarshal(head, &m); err != nil {
			return ""
		}
		v, _ := m[field].(string)
		v = strings.ToLower(strings.TrimSpace(v))
		if v == "" {
			return ""
		}
		sum := sha256.Sum256([]byte(v))
		return hex.EncodeToString(sum[:16])
	}
}

type readCloser struct {
	io.Reader
	io.Closer
}
