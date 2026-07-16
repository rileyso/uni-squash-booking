package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

type AdminSession struct {
	Token   string
	CSRF    string
	Expires time.Time
}

func (s *Service) AdminEnabled() bool { return s.adminUsername != "" && s.adminPasswordHash != "" }

func (s *Service) AdminSignIn(ctx context.Context, username, password string) (AdminSession, error) {
	verified := verifyPIN(s.adminPasswordHash, password)
	if !verified || strings.TrimSpace(username) != s.adminUsername {
		return AdminSession{}, ErrUnauthenticated
	}
	token, csrf := make([]byte, 32), make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return AdminSession{}, err
	}
	if _, err := rand.Read(csrf); err != nil {
		return AdminSession{}, err
	}
	expires := s.now().Add(30 * time.Minute)
	if err := s.store.CreateAdminSession(ctx, token, csrf, s.now(), expires); err != nil {
		return AdminSession{}, err
	}
	return AdminSession{Token: base64.RawURLEncoding.EncodeToString(token), CSRF: base64.RawURLEncoding.EncodeToString(csrf), Expires: expires}, nil
}

func (s *Service) AdminForToken(ctx context.Context, value string) (string, error) {
	token, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return "", ErrUnauthenticated
	}
	csrf, err := s.store.AdminSession(ctx, token, s.now())
	if err != nil {
		return "", ErrUnauthenticated
	}
	return base64.RawURLEncoding.EncodeToString(csrf), nil
}

func (s *Service) AdminSignOut(ctx context.Context, value string) {
	token, err := base64.RawURLEncoding.DecodeString(value)
	if err == nil {
		_ = s.store.DeleteAdminSession(ctx, token)
	}
}

func (s *Service) SearchAccounts(ctx context.Context, query string) ([]sqlite.AdminAccountResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || len(query) > 80 {
		return nil, ErrInvalidInput
	}
	return s.store.SearchAccounts(ctx, query)
}

func (s *Service) ResetMemberPIN(ctx context.Context, accountID int64, attested bool) (string, error) {
	if !s.config.Capabilities.PINReset || accountID <= 0 || !attested {
		return "", ErrInvalidInput
	}
	temporaryPIN, err := randomCode()
	if err != nil {
		return "", err
	}
	pinHash, err := hashPIN(temporaryPIN)
	if err != nil {
		return "", err
	}
	if err := s.store.ResetMemberPIN(ctx, accountID, pinHash); err != nil {
		return "", fmt.Errorf("reset member PIN: %w", err)
	}
	return temporaryPIN, nil
}
