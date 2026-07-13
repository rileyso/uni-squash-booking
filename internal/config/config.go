package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

type Environment string

const (
	Development Environment = "development"
	Test        Environment = "test"
	Production  Environment = "production"
)

type Approval struct {
	Status     string `json:"status"`
	ApprovedBy string `json:"approved_by"`
	ApprovedOn string `json:"approved_on"`
}

func (a Approval) Valid() bool {
	if a.Status != "approved" || strings.TrimSpace(a.ApprovedBy) == "" {
		return false
	}
	_, err := time.Parse("2006-01-02", a.ApprovedOn)
	return err == nil
}

type ApprovalManifest struct {
	PublicNamedAttendance Approval `json:"public_named_attendance"`
	PublicAccountCreation Approval `json:"public_account_creation"`
	PINResetEvidence      Approval `json:"pin_reset_evidence"`
	RetentionAndDeletion  Approval `json:"retention_and_deletion"`
}

type Capabilities struct {
	PublicNamedAttendance bool
	PublicAccountCreation bool
	PINReset              bool
	SelfDelete            bool
}

type Config struct {
	Environment        Environment
	Address            string
	DatabasePath       string
	RecoveryGeneration string
	Synthetic          bool
	Capabilities       Capabilities
}

func Load(getenv func(string) string) (Config, error) {
	environment := Environment(strings.TrimSpace(getenv("APP_ENV")))
	if environment == "" {
		environment = Development
	}
	if environment != Development && environment != Test && environment != Production {
		return Config{}, fmt.Errorf("APP_ENV: unknown value %q", environment)
	}

	address := strings.TrimSpace(getenv("APP_ADDR"))
	if address == "" {
		address = "127.0.0.1:8080"
	}
	databasePath := strings.TrimSpace(getenv("DATABASE_PATH"))
	if databasePath == "" {
		databasePath = "data/squash.sqlite"
	}
	recoveryGeneration := strings.TrimSpace(getenv("RECOVERY_GENERATION"))
	if recoveryGeneration == "" && environment != Production {
		recoveryGeneration = "synthetic-development-generation"
	}

	config := Config{Environment: environment, Address: address, DatabasePath: databasePath, RecoveryGeneration: recoveryGeneration, Synthetic: environment != Production}
	if environment != Production {
		if !isLoopback(address) {
			return Config{}, errors.New("development and test must bind to loopback")
		}
		return config, nil
	}
	if recoveryGeneration == "" {
		return Config{}, errors.New("RECOVERY_GENERATION is required in production")
	}
	manifestPath := strings.TrimSpace(getenv("APPROVAL_MANIFEST"))
	if manifestPath == "" {
		return Config{}, errors.New("APPROVAL_MANIFEST is required in production")
	}
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return Config{}, err
	}
	config.Capabilities = Capabilities{
		PublicNamedAttendance: manifest.PublicNamedAttendance.Valid(),
		PINReset:              manifest.PINResetEvidence.Valid(),
		SelfDelete:            manifest.RetentionAndDeletion.Valid(),
	}
	config.Capabilities.PublicAccountCreation = manifest.PublicAccountCreation.Valid() && config.Capabilities.PINReset
	return config, nil
}

func readManifest(path string) (ApprovalManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ApprovalManifest{}, fmt.Errorf("read approval manifest: %w", err)
	}
	var manifest ApprovalManifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return ApprovalManifest{}, fmt.Errorf("decode approval manifest: %w", err)
	}
	return manifest, nil
}

func isLoopback(address string) bool {
	return strings.HasPrefix(address, "127.0.0.1:") || strings.HasPrefix(address, "localhost:") || strings.HasPrefix(address, "[::1]:")
}
