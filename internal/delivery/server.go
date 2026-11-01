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
	mux.HandleFunc("POST /auth/register", h.Register)
	mux.HandleFunc("POST /auth/login", h.Login)
	mux.HandleFunc("POST /auth/logout", h.Logout)
	// Butuh authToken
	mux.Handle("GET /auth/me", requireAuth(h.Me))
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

	// Middleware global: transaction-id di semua response.
	return middleware.Chain(mux, middleware.TransactionID), nil
}
