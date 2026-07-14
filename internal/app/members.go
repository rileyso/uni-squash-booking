package app

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/domain"
	"github.com/rileyso/uni-squash-booking/internal/sqlite"
	"golang.org/x/crypto/argon2"
)

var (
	ErrInvalidInput    = errors.New("invalid input")
	ErrUnauthenticated = errors.New("sign in required")
	ErrConflict        = errors.New("attendance conflicts with the current schedule")
)

type Member struct {
	ID                        int64
	FullUsername, DisplayName string
}
type Session struct {
	Token, CSRF string
	Expires     time.Time
	Member      Member
}
type Plan struct {
	Date, DateLabel, StartLabel, EndLabel string
	StartMinute, EndMinute                int
}

type PlayableSegment struct{ StartMinute, EndMinute int }

func (s *Service) PlayableSegments(ctx context.Context, dateValue string) ([]PlayableSegment, error) {
	date, err := domain.ParseCivilDate(dateValue)
	if err != nil {
		return nil, ErrInvalidInput
	}
	data, err := s.store.LoadAnonymousTimetable(ctx, date, date)
	if err != nil {
		return nil, err
	}
	var segments []PlayableSegment
	for minute := 600; minute < 1320; minute += 30 {
		bucket, _ := domain.NewInterval(minute, minute+30)
		one := courtState(data.Weekly, data.OneOffs, date, 1, bucket, s.location)
		two := courtState(data.Weekly, data.OneOffs, date, 2, bucket, s.location)
		open := one.Class == "open" || two.Class == "open"
		if open && (len(segments) == 0 || segments[len(segments)-1].EndMinute != minute) {
			segments = append(segments, PlayableSegment{StartMinute: minute, EndMinute: minute + 30})
		} else if open {
			segments[len(segments)-1].EndMinute = minute + 30
		}
	}
	return segments, nil
}

func (s *Service) IdentityEnabled() bool {
	return s.config.Synthetic || s.config.Capabilities.PublicAccountCreation
}

func (s *Service) CreateAccount(ctx context.Context, displayName, pin, memberStatus string, privacy bool) (Session, error) {
	displayName = strings.TrimSpace(displayName)
	if !s.IdentityEnabled() {
		return Session{}, ErrUnauthenticated
	}
	if len(displayName) < 2 || len(displayName) > 40 || !validPIN(pin) || !privacy || (memberStatus != "member" && memberStatus != "visitor") {
		return Session{}, ErrInvalidInput
	}
	pinHash, err := hashPIN(pin)
	if err != nil {
		return Session{}, err
	}
	handle := handleName(displayName)
	for attempts := 0; attempts < 32; attempts++ {
		code, err := randomCode()
		if err != nil {
			return Session{}, err
		}
		account, err := s.store.CreateAccount(ctx, code, handle+"#"+code, displayName, pinHash, memberStatus, s.now())
		if err != nil {
			continue
		}
		return s.newSession(ctx, account)
	}
	return Session{}, fmt.Errorf("allocate player code")
}

func (s *Service) SignIn(ctx context.Context, username, pin string) (Session, error) {
	account, err := s.store.AccountByUsername(ctx, strings.TrimSpace(username))
	if err != nil || account.Status != "active" || !verifyPIN(account.PINHash, pin) {
		return Session{}, ErrUnauthenticated
	}
	return s.newSession(ctx, account)
}

func (s *Service) newSession(ctx context.Context, account sqlite.Account) (Session, error) {
	token, csrf := make([]byte, 32), make([]byte, 32)
	if _, err := rand.Read(token); err != nil {
		return Session{}, err
	}
	if _, err := rand.Read(csrf); err != nil {
		return Session{}, err
	}
	expires := s.now().Add(7 * 24 * time.Hour)
	if err := s.store.CreateMemberSession(ctx, account.ID, token, csrf, s.now(), expires); err != nil {
		return Session{}, err
	}
	return Session{Token: base64.RawURLEncoding.EncodeToString(token), CSRF: base64.RawURLEncoding.EncodeToString(csrf), Expires: expires, Member: memberView(account)}, nil
}

func (s *Service) MemberForToken(ctx context.Context, value string) (Member, string, error) {
	token, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return Member{}, "", ErrUnauthenticated
	}
	account, csrf, err := s.store.AccountBySession(ctx, token, s.now())
	if err != nil || account.Status != "active" {
		return Member{}, "", ErrUnauthenticated
	}
	return memberView(account), base64.RawURLEncoding.EncodeToString(csrf), nil
}

func (s *Service) SignOut(ctx context.Context, value string) {
	token, err := base64.RawURLEncoding.DecodeString(value)
	if err == nil {
		_ = s.store.DeleteMemberSession(ctx, token)
	}
}

func (s *Service) Plans(ctx context.Context, member Member) ([]Plan, error) {
	today := domain.CivilDateFromTime(s.now(), s.location)
	rows, err := s.store.PlansForAccount(ctx, member.ID, today.String(), today.AddDays(13, s.location).String())
	if err != nil {
		return nil, err
	}
	return s.planViews(rows), nil
}

func (s *Service) PlanForDate(ctx context.Context, member Member, date string) (*Plan, error) {
	row, err := s.store.PlanForAccountDate(ctx, member.ID, date)
	if sqlite.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	plans := s.planViews([]sqlite.AttendancePlan{row})
	return &plans[0], nil
}

func (s *Service) SaveAttendance(ctx context.Context, member Member, dateValue string, start, end int) error {
	if err := s.ValidateAttendance(ctx, dateValue, start, end); err != nil {
		return err
	}
	return s.store.UpsertAttendance(ctx, member.ID, dateValue, start, end, s.now())
}

func (s *Service) ValidateAttendance(ctx context.Context, dateValue string, start, end int) error {
	date, err := domain.ParseCivilDate(dateValue)
	if err != nil {
		return ErrInvalidInput
	}
	interval, err := domain.NewInterval(start, end)
	if err != nil {
		return ErrInvalidInput
	}
	if start < 600 || end > 1320 || end-start < 60 {
		return ErrInvalidInput
	}
	today := domain.CivilDateFromTime(s.now(), s.location)
	if date.Time(s.location).Before(today.Time(s.location)) || date.Time(s.location).After(today.AddDays(13, s.location).Time(s.location)) {
		return ErrInvalidInput
	}
	localNow := s.now().In(s.location)
	if date == today && start < localNow.Hour()*60+localNow.Minute() {
		return ErrInvalidInput
	}
	data, err := s.store.LoadAnonymousTimetable(ctx, date, date)
	if err != nil {
		return err
	}
	for minute := interval.StartMinute; minute < interval.EndMinute; minute += 30 {
		bucket, _ := domain.NewInterval(minute, minute+30)
		one := courtState(data.Weekly, data.OneOffs, date, 1, bucket, s.location)
		two := courtState(data.Weekly, data.OneOffs, date, 2, bucket, s.location)
		if one.Class != "open" && two.Class != "open" {
			return ErrConflict
		}
	}
	return nil
}

func (s *Service) RemoveAttendance(ctx context.Context, member Member, date string) error {
	return s.store.RemoveAttendance(ctx, member.ID, date)
}

func (s *Service) planViews(rows []sqlite.AttendancePlan) []Plan {
	result := make([]Plan, 0, len(rows))
	for _, row := range rows {
		d, _ := domain.ParseCivilDate(row.Date)
		result = append(result, Plan{Date: row.Date, DateLabel: d.Time(s.location).Format("Mon 2 Jan"), StartMinute: row.StartMinute, EndMinute: row.EndMinute, StartLabel: minuteLabel(row.StartMinute), EndLabel: minuteLabel(row.EndMinute)})
	}
	return result
}

func memberView(a sqlite.Account) Member {
	return Member{ID: a.ID, FullUsername: a.FullUsername, DisplayName: a.DisplayName}
}
func validPIN(pin string) bool { ok, _ := regexp.MatchString(`^[0-9]{4}$`, pin); return ok }
func handleName(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(name) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		return "player"
	}
	return b.String()
}
func randomCode() (string, error) {
	var b [2]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return fmt.Sprintf("%04d", (int(b[0])<<8|int(b[1]))%10000), nil
}
func hashPIN(pin string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	sum := argon2.IDKey([]byte(pin), salt, 1, 64*1024, 2, 32)
	return "argon2id$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(sum), nil
}
func verifyPIN(encoded, pin string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 3 || parts[0] != "argon2id" {
		return false
	}
	salt, e1 := base64.RawStdEncoding.DecodeString(parts[1])
	want, e2 := base64.RawStdEncoding.DecodeString(parts[2])
	if e1 != nil || e2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(pin), salt, 1, 64*1024, 2, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}
