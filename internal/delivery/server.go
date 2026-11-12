package delivery

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"

	"github.com/refreshTechRusuh/user-service/internal/config"
	"github.com/refreshTechRusuh/user-service/internal/user/application/usecase"
	"github.com/refreshTechRusuh/user-service/internal/user/infrastructure/cache"
	"github.com/refreshTechRusuh/user-service/internal/user/infrastructure/db"
	"github.com/refreshTechRusuh/user-service/internal/user/infrastructure/gateway"
	handler "github.com/refreshTechRusuh/user-service/internal/user/infrastructure/http"
	"github.com/refreshTechRusuh/user-service/pkg/middleware"
	"github.com/refreshTechRusuh/user-service/pkg/token"
)

// New merakit semua lapis + daftarkan route. Mengembalikan handler (mux dibungkus
// middleware global) agar siap dipasang ke http.Server. Juga memastikan akun admin
// awal ada (bila ADMIN_EMAIL di-set) — gagal = service tidak start (fail fast).
func New(ctx context.Context, cfg *config.Config, pool *pgxpool.Pool, rdb *goredis.Client) (http.Handler, error) {
	// Adapter
	repo := db.NewUserRepository(pool)
	tokenStore := cache.NewTokenStore(rdb)

	// Generator JWT dari config (secret + TTL).
	tokenGen := token.NewGenerator(
		cfg.JWT.AccessSecret, cfg.JWT.RefreshSecret,
		cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL,
	)

	uc := usecase.NewUserUsecase(repo, tokenGen, tokenStore)
	h := handler.NewUserHandler(uc)

	// Akun admin awal (credential dari env, bukan dari repo).
	res, err := uc.EnsureAdmin(ctx, cfg.Admin.Email, cfg.Admin.Password)
	if err != nil {
		return nil, fmt.Errorf("bootstrap admin: %w", err)
	}
	switch res {
	case usecase.BootstrapCreated:
		slog.Info("akun admin dibuat", "email", cfg.Admin.Email)
	case usecase.BootstrapNotAdm:
		slog.Warn("ADMIN_EMAIL sudah terdaftar sebagai customer; tidak dinaikkan otomatis", "email", cfg.Admin.Email)
	}

	// Member/NIK: repo + client Dukcapil + usecase + handler.
	memberRepo := db.NewMemberRepository(pool)
	dukcapilClient := gateway.NewDukcapilClient(
		cfg.Dukcapil.BaseURL,
		time.Duration(cfg.Dukcapil.TimeoutSeconds)*time.Second,
	)
	memberUC := usecase.NewMemberUsecase(memberRepo, dukcapilClient)
	mh := handler.NewMemberHandler(memberUC)

	// Verifikasi NIK publik (FE tidak boleh langsung ke dukcapil-mock) + rate limit Redis.
	limiter := cache.NewRateLimiter(rdb)
	vh := handler.NewVerifyHandler(memberUC, limiter,
		cfg.Verify.RatePerIPPerMin, cfg.Verify.RatePerNIKPerMin)

	// Riwayat tiket (read-only lintas tabel booking/event).
	historyRepo := db.NewHistoryRepository(pool)
	historyUC := usecase.NewHistoryUsecase(historyRepo)
	hh := handler.NewHistoryHandler(historyUC)

	// requireAuth = helper bungkus handler dengan middleware wajib-login.
	requireAuth := func(next http.HandlerFunc) http.Handler {
		return middleware.Chain(http.HandlerFunc(next), middleware.RequireAuth(tokenGen))
	}

	mux := http.NewServeMux()
	// Publik
	// Login & register rawan brute-force / pembuatan akun massal -> dibatasi.
	// Login punya dua lapis: per IP dan per email (lapis per-email tetap berlaku
	// walau penyerang memalsukan X-Forwarded-For).
	sec := cfg.Security
	mux.Handle("POST /auth/register", middleware.Chain(http.HandlerFunc(h.Register),
		middleware.RateLimit(limiter, "register:ip", sec.RegisterPerIPPerMin, time.Minute, middleware.ClientIPKey),
	))
	mux.Handle("POST /auth/login", middleware.Chain(http.HandlerFunc(h.Login),
		middleware.RateLimit(limiter, "login:ip", sec.LoginPerIPPerMin, time.Minute, middleware.ClientIPKey),
		middleware.RateLimit(limiter, "login:email", sec.LoginPerEmailPerMin, time.Minute, middleware.BodyFieldKey("email")),
	))
	mux.HandleFunc("POST /auth/logout", h.Logout)
	// Butuh authToken
	mux.Handle("GET /auth/me", requireAuth(h.Me))
	// Verifikasi NIK ke Dukcapil (publik: guest ikut memakai; dibatasi rate limit)
	mux.HandleFunc("POST /api/user/v1/nik/verify", vh.Verify)
	// Member/NIK (semua butuh authToken, scoped per user)
	mux.Handle("POST /api/user/v1/member/create", requireAuth(mh.Create))
	mux.Handle("GET /api/user/v1/member/list", requireAuth(mh.List))
	mux.Handle("PUT /api/user/v1/member/update/{id}", requireAuth(mh.Update))
	mux.Handle("DELETE /api/user/v1/member/delete/{id}", requireAuth(mh.Delete))
	// Riwayat tiket (butuh authToken)
	mux.Handle("GET /api/user/v1/ticket-history", requireAuth(hh.List))
	// Health
	mux.HandleFunc("GET /sys/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"00000","message":"user-service ok","data":{}}`))
	})

	// Middleware global (luar -> dalam): header keamanan, CORS, batas body, tracing, transaction-id.
	//  - header keamanan paling luar agar ada di SEMUA response (termasuk preflight & 413)
	//  - batas body di dalam CORS supaya respons 413 tetap membawa header CORS
	//    (kalau tidak, browser melaporkan "CORS error" alih-alih 413)
	//  - CORS di luar tracing agar preflight OPTIONS tidak membuat span
	return middleware.Chain(mux,
		middleware.SecurityHeaders,
		middleware.CORSMiddleware("*"),
		middleware.MaxBody(sec.MaxBodyBytes),
		middleware.OtelMiddleware("user-service", "/sys/ping"),
		middleware.TransactionID,
	), nil
}
