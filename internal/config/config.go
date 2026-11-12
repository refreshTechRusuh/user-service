// Package config memuat konfigurasi service dari environment variable.
// Pola sama dengan dukcapil-mock, tapi user-service butuh dua bagian tambahan:
//   - JWT   : untuk menerbitkan & memverifikasi authToken/refreshToken
//   - Redis : untuk menyimpan state refresh-token (logout = revoke)
//
// Pola mengikuti service lain (event-service) agar konsisten lintas tim.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config = seluruh konfigurasi service, dikelompokkan per bagian.
type Config struct {
	App      AppConfig
	Log      LogConfig
	DB       DBConfig
	JWT      JWTConfig
	Redis    RedisConfig
	Dukcapil DukcapilConfig
	Admin    AdminBootstrapConfig
	Verify   VerifyNIKConfig
}

// VerifyNIKConfig = batas panggilan endpoint publik verifikasi NIK (per menit; 0 = mati).
type VerifyNIKConfig struct {
	RatePerIPPerMin  int // NIK_VERIFY_RATE_PER_IP_PER_MIN
	RatePerNIKPerMin int // NIK_VERIFY_RATE_PER_NIK_PER_MIN
}

type AppConfig struct {
	Env  string
	Port int
}

type LogConfig struct {
	Level  string
	Format string
}

type DBConfig struct {
	Host            string
	Port            int
	User            string
	Password        string
	Name            string
	SSLMode         string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnTimeoutSecs int
}

// JWTConfig = rahasia & masa berlaku token LOGIN (authToken + refreshToken).
//
// AccessSecret (env USER_JWT_SECRET) SENGAJA berbeda dari ACCESS_TOKEN_SECRET milik
// queue-service (Access Token antrian). Alasannya:
//   - token login (berisi role, bisa admin) dan token antrian tidak bisa saling
//     menggantikan walau sama-sama JWT HS256;
//   - queue-service tidak memegang secret yang bisa membuat token admin.
//
// Service lain yang memverifikasi token login (event/booking/payment) memakai
// USER_JWT_SECRET yang sama.
// Field secret diberi tag json:"-" agar tak ikut ter-log bila Config di-marshal.
type JWTConfig struct {
	AccessSecret  string        `json:"-"` // USER_JWT_SECRET
	RefreshSecret string        `json:"-"` // REFRESH_TOKEN_SECRET
	AccessTTL     time.Duration // umur authToken (mis. 1 jam)
	RefreshTTL    time.Duration // umur refreshToken (mis. 7 hari)
}

// AdminBootstrapConfig = akun admin awal yang dibuat saat startup bila belum ada.
// Kosong = dilewati. Credential TIDAK disimpan di repo/migration; isi lewat env /
// secret manager.
type AdminBootstrapConfig struct {
	Email    string
	Password string `json:"-"`
}

type RedisConfig struct {
	Addr     string
	Password string `json:"-"`
	DB       int
}

// DukcapilConfig = alamat service dukcapil-mock yang dipanggil user-service.
type DukcapilConfig struct {
	BaseURL        string
	TimeoutSeconds int
}

// Load membaca .env (bila ada) lalu environment, dengan default aman untuk dev.
func Load() (*Config, error) {
	_ = godotenv.Load() // .env opsional; abaikan error bila file tak ada

	cfg := &Config{
		App: AppConfig{
			Env:  getEnv("APP_ENV", "local"),
			Port: getEnvInt("APP_PORT", 8086), // 8086 = port user-service (dukcapil 8085)
		},
		Log: LogConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		DB: DBConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnvInt("DB_PORT", 5432),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", "password"),
			Name:            getEnv("DB_NAME", "rusuh_booking"),
			SSLMode:         getEnv("DB_SSL_MODE", "disable"),
			MaxOpenConns:    getEnvInt("DB_MAX_OPEN_CONNS", 10),
			MaxIdleConns:    getEnvInt("DB_MAX_IDLE_CONNS", 5),
			ConnTimeoutSecs: getEnvInt("DB_CONN_TIMEOUT_SECONDS", 5),
		},
		JWT: JWTConfig{
			AccessSecret:  getEnv("USER_JWT_SECRET", defaultUserJWTSecret),
			RefreshSecret: getEnv("REFRESH_TOKEN_SECRET", defaultRefreshSecret),
			AccessTTL:     time.Duration(getEnvInt("USER_JWT_TTL_SECONDS", 3600)) * time.Second,
			RefreshTTL:    time.Duration(getEnvInt("REFRESH_TOKEN_TTL_SECONDS", 7*24*3600)) * time.Second,
		},
		Redis: RedisConfig{
			Addr:     getEnv("REDIS_ADDR", "localhost:6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},
		Dukcapil: DukcapilConfig{
			BaseURL:        getEnv("DUKCAPIL_BASE_URL", "http://localhost:8085"),
			TimeoutSeconds: getEnvInt("DUKCAPIL_TIMEOUT_SECONDS", 5),
		},
		Admin: AdminBootstrapConfig{
			Email:    getEnv("ADMIN_EMAIL", ""),
			Password: getEnv("ADMIN_PASSWORD", ""),
		},
		Verify: VerifyNIKConfig{
			RatePerIPPerMin:  getEnvInt("NIK_VERIFY_RATE_PER_IP_PER_MIN", 10),
			RatePerNIKPerMin: getEnvInt("NIK_VERIFY_RATE_PER_NIK_PER_MIN", 5),
		},
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Default secret hanya untuk dev lokal. Di production wajib diganti (dicek validate).
const (
	// Default KHUSUS dev lokal; validate() menolaknya saat APP_ENV=production (ada tesnya).
	defaultUserJWTSecret = "dev-user-jwt-secret-change-in-production" // #nosec G101 -- default dev, ditolak di production
	defaultRefreshSecret = "dev-refresh-secret-change-in-production"  // #nosec G101 -- default dev, ditolak di production
)

// validate menolak konfigurasi berbahaya sebelum service jalan (fail fast).
func (c *Config) validate() error {
	if c.JWT.AccessSecret == c.JWT.RefreshSecret {
		return fmt.Errorf("config: USER_JWT_SECRET dan REFRESH_TOKEN_SECRET harus berbeda")
	}
	if c.App.Env == "production" {
		if c.JWT.AccessSecret == defaultUserJWTSecret || c.JWT.RefreshSecret == defaultRefreshSecret {
			return fmt.Errorf("config: USER_JWT_SECRET & REFRESH_TOKEN_SECRET wajib di-set di production")
		}
	}
	if (c.Admin.Email == "") != (c.Admin.Password == "") {
		return fmt.Errorf("config: ADMIN_EMAIL dan ADMIN_PASSWORD harus diisi berdua atau dikosongkan berdua")
	}
	return nil
}

// DSN membentuk connection string PostgreSQL dari DBConfig.
func (d DBConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s connect_timeout=%d",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode, d.ConnTimeoutSecs,
	)
}

// SetupLogger memasang slog global sesuai LogConfig (json/text + level).
func SetupLogger(cfg LogConfig) {
	level := slog.LevelInfo
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}

	var h slog.Handler
	if cfg.Format == "text" {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		h = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(h))
}

// --- helper baca env dengan fallback ---

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return fallback
}
