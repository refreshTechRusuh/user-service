package config

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ShouldUseSafeDefaults_WhenEnvEmpty(t *testing.T) {
	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, 8086, cfg.App.Port)
	assert.Equal(t, "local", cfg.App.Env)
	assert.Equal(t, "info", cfg.Log.Level)
	assert.Equal(t, "json", cfg.Log.Format)
	assert.Equal(t, time.Hour, cfg.JWT.AccessTTL)
	assert.Equal(t, 7*24*time.Hour, cfg.JWT.RefreshTTL)
	assert.Equal(t, 10, cfg.Verify.RatePerIPPerMin)
	assert.Equal(t, 5, cfg.Verify.RatePerNIKPerMin)
	assert.Equal(t, 5, cfg.Dukcapil.TimeoutSeconds)
}

func TestLoad_ShouldReadEnvOverrides(t *testing.T) {
	t.Setenv("APP_PORT", "9999")
	t.Setenv("DB_HOST", "10.0.0.5")
	t.Setenv("NIK_VERIFY_RATE_PER_IP_PER_MIN", "3")
	t.Setenv("NIK_VERIFY_RATE_PER_NIK_PER_MIN", "0")
	t.Setenv("DUKCAPIL_BASE_URL", "https://dukcapil.example")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, 9999, cfg.App.Port)
	assert.Equal(t, "10.0.0.5", cfg.DB.Host)
	assert.Equal(t, 3, cfg.Verify.RatePerIPPerMin)
	assert.Equal(t, 0, cfg.Verify.RatePerNIKPerMin, "0 = limiter dimatikan")
	assert.Equal(t, "https://dukcapil.example", cfg.Dukcapil.BaseURL)
}

func TestLoad_ShouldRejectProductionWithDefaultSecrets(t *testing.T) {
	t.Setenv("APP_ENV", "production")

	_, err := Load()

	assert.Error(t, err, "production tidak boleh jalan dengan secret bawaan dev")
}

func TestGetEnvInt_ShouldFallBack_WhenNotANumber(t *testing.T) {
	t.Setenv("ANGKA_RUSAK", "abc")
	t.Setenv("ANGKA_BAGUS", "42")

	assert.Equal(t, 7, getEnvInt("ANGKA_RUSAK", 7))
	assert.Equal(t, 42, getEnvInt("ANGKA_BAGUS", 7))
	assert.Equal(t, 7, getEnvInt("TIDAK_ADA", 7))
}

func TestDSN_ShouldContainAllConnectionParts(t *testing.T) {
	d := DBConfig{Host: "h", Port: 5432, User: "u", Password: "p", Name: "n", SSLMode: "require", ConnTimeoutSecs: 3}

	assert.Equal(t, "host=h port=5432 user=u password=p dbname=n sslmode=require connect_timeout=3", d.DSN())
}

func TestSetupLogger_ShouldApplyLevel(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	cases := map[string]slog.Level{
		"debug": slog.LevelDebug,
		"warn":  slog.LevelWarn,
		"error": slog.LevelError,
		"info":  slog.LevelInfo,
		"xxx":   slog.LevelInfo, // tak dikenal → info
	}
	for name, want := range cases {
		t.Run(name, func(t *testing.T) {
			SetupLogger(LogConfig{Level: name, Format: "json"})
			ctx := context.Background()
			assert.Equal(t, want <= slog.LevelDebug, slog.Default().Enabled(ctx, slog.LevelDebug))
			assert.Equal(t, want <= slog.LevelInfo, slog.Default().Enabled(ctx, slog.LevelInfo))
			assert.Equal(t, want <= slog.LevelWarn, slog.Default().Enabled(ctx, slog.LevelWarn))
			assert.True(t, slog.Default().Enabled(ctx, slog.LevelError))
		})
	}
}

func TestSetupLogger_ShouldSupportTextFormat(t *testing.T) {
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })

	SetupLogger(LogConfig{Level: "info", Format: "text"})

	assert.True(t, slog.Default().Enabled(context.Background(), slog.LevelInfo))
}

func TestLoad_Security_ShouldUseSafeDefaults(t *testing.T) {
	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, int64(1<<20), cfg.Security.MaxBodyBytes)
	assert.Equal(t, 10, cfg.Security.LoginPerIPPerMin)
	assert.Equal(t, 10, cfg.Security.LoginPerEmailPerMin)
	assert.Equal(t, 5, cfg.Security.RegisterPerIPPerMin)
}

func TestLoad_Security_ShouldReadEnvOverrides(t *testing.T) {
	t.Setenv("MAX_BODY_BYTES", "2048")
	t.Setenv("LOGIN_RATE_PER_IP_PER_MIN", "20")
	t.Setenv("LOGIN_RATE_PER_EMAIL_PER_MIN", "0")
	t.Setenv("REGISTER_RATE_PER_IP_PER_MIN", "3")

	cfg, err := Load()

	require.NoError(t, err)
	assert.Equal(t, int64(2048), cfg.Security.MaxBodyBytes)
	assert.Equal(t, 20, cfg.Security.LoginPerIPPerMin)
	assert.Equal(t, 0, cfg.Security.LoginPerEmailPerMin, "0 = dimatikan")
	assert.Equal(t, 3, cfg.Security.RegisterPerIPPerMin)
}
