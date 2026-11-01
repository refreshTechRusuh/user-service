package config

import "testing"

func validConfig() *Config {
	return &Config{
		App: AppConfig{Env: "local"},
		JWT: JWTConfig{AccessSecret: defaultUserJWTSecret, RefreshSecret: defaultRefreshSecret},
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(c *Config)
		wantErr bool
	}{
		{"default lokal → ok", func(c *Config) {}, false},
		{"secret access = refresh → ditolak", func(c *Config) { c.JWT.RefreshSecret = c.JWT.AccessSecret }, true},
		{"production pakai secret default → ditolak", func(c *Config) { c.App.Env = "production" }, true},
		{"production secret custom → ok", func(c *Config) {
			c.App.Env = "production"
			c.JWT.AccessSecret = "prod-user-secret"
			c.JWT.RefreshSecret = "prod-refresh-secret"
		}, false},
		{"ADMIN_EMAIL tanpa password → ditolak", func(c *Config) { c.Admin.Email = "admin@rusuh.local" }, true},
		{"ADMIN_PASSWORD tanpa email → ditolak", func(c *Config) { c.Admin.Password = "x" }, true},
		{"admin lengkap → ok", func(c *Config) {
			c.Admin.Email = "admin@rusuh.local"
			c.Admin.Password = "admin-password-123"
		}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg := validConfig()
			c.mutate(cfg)
			if err := cfg.validate(); (err != nil) != c.wantErr {
				t.Errorf("validate() err=%v; mau error=%v", err, c.wantErr)
			}
		})
	}
}

func TestLoad_ShouldReadUserJWTSecret_NotQueueAccessTokenSecret(t *testing.T) {
	t.Setenv("ACCESS_TOKEN_SECRET", "queue-secret")
	t.Setenv("USER_JWT_SECRET", "user-secret")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JWT.AccessSecret != "user-secret" {
		t.Errorf("AccessSecret = %q; mau user-secret (bukan ACCESS_TOKEN_SECRET milik queue)", cfg.JWT.AccessSecret)
	}
}
