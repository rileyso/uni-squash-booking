package config

import (
	"os"
	"path/filepath"
	"testing"
)

func env(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestDevelopmentIsSyntheticAndLoopbackOnly(t *testing.T) {
	config, err := Load(env(map[string]string{}))
	if err != nil {
		t.Fatal(err)
	}
	if !config.Synthetic || config.Environment != Development || config.Address != "127.0.0.1:18080" {
		t.Fatalf("unexpected config: %#v", config)
	}
	if _, err := Load(env(map[string]string{"APP_ENV": "test", "APP_ADDR": "0.0.0.0:8080"})); err == nil {
		t.Fatal("non-loopback synthetic server was accepted")
	}
}

func TestProductionFailsClosed(t *testing.T) {
	if _, err := Load(env(map[string]string{"APP_ENV": "production"})); err == nil {
		t.Fatal("production without recovery generation was accepted")
	}
}

func TestPublicCreationAlsoRequiresRecoveryApproval(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "approvals.json")
	data := []byte(`{"public_named_attendance":{"status":"approved","approved_by":"Committee","approved_on":"2026-07-14"},"public_account_creation":{"status":"approved","approved_by":"Committee","approved_on":"2026-07-14"},"pin_reset_evidence":{"status":"pending"},"retention_and_deletion":{"status":"pending"}}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := Load(env(map[string]string{"APP_ENV": "production", "RECOVERY_GENERATION": "generation-1", "APPROVAL_MANIFEST": path}))
	if err != nil {
		t.Fatal(err)
	}
	if !config.Capabilities.PublicNamedAttendance || config.Capabilities.PublicAccountCreation {
		t.Fatalf("unexpected capabilities: %#v", config.Capabilities)
	}
}
