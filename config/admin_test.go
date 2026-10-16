package config

import "testing"

func lookupFrom(env map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := env[key]
		return v, ok
	}
}

func TestDefaultsShipNoAdminCredential(t *testing.T) {
	cfg, _ := Load("")

	if cfg.Admin.Enabled() {
		t.Fatalf("defaults enable admin with %q/%q", cfg.Admin.Username, cfg.Admin.Password)
	}
}

func TestAdminCredentialComesFromEnvironment(t *testing.T) {
	cfg, _ := Load("")

	ApplyEnv(cfg, lookupFrom(map[string]string{
		"CATACOMBS_ADMIN_USERNAME": "ops",
		"CATACOMBS_ADMIN_PASSWORD": "s3cret-pass",
	}))

	if !cfg.Admin.Enabled() || cfg.Admin.Username != "ops" || cfg.Admin.Password != "s3cret-pass" {
		t.Fatalf("admin not taken from environment: %+v", cfg.Admin)
	}
}

func TestAdminStaysDisabledWithPartialEnvironment(t *testing.T) {
	cfg, _ := Load("")

	ApplyEnv(cfg, lookupFrom(map[string]string{"CATACOMBS_ADMIN_USERNAME": "ops"}))

	if cfg.Admin.Enabled() {
		t.Fatal("admin enabled without a password")
	}
}
