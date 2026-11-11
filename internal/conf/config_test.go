package conf

import "testing"

func TestProductionRejectsEphemeralDemo(t *testing.T) {
	t.Setenv("APP_ENV", "prod")
	t.Setenv("DEMO_MODE", "true")
	t.Setenv("MYSQL_PASSWORD", "test-only")
	if _, err := Load(); err == nil {
		t.Fatal("production must not use ephemeral demo records")
	}
}

func TestShortAdminTokenRejected(t *testing.T) {
	t.Setenv("APP_ENV", "test")
	t.Setenv("DEMO_MODE", "false")
	t.Setenv("ADMIN_TOKEN", "short")
	if _, err := Load(); err == nil {
		t.Fatal("weak admin token accepted")
	}
}
