package delivery

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/refreshTechRusuh/user-service/internal/user/application/usecase"
	"github.com/refreshTechRusuh/user-service/internal/user/infrastructure/db"
	handler "github.com/refreshTechRusuh/user-service/internal/user/infrastructure/http"
)

func New(pool *pgxpool.Pool) *http.ServeMux {
	repo := db.NewUserRepository(pool)

	usecase := usecase.NewUserUsecase(repo)

	handler := handler.NewUserHandler(usecase)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/register", handler.Register)
	mux.HandleFunc("GET /sys/ping", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"00000","message":"user-service ok","data":{}}`))
	})
	return mux
}
