package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/domain"
	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

func TestAccountSessionAndMultipleDailyPlans(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	session, err := service.CreateAccount(ctx, "Test Player", "2468", "member", true)
	if err != nil {
		t.Fatal(err)
	}
	member, csrf, err := service.MemberForToken(ctx, session.Token)
	if err != nil || csrf == "" || member.FullUsername == "" {
		t.Fatalf("session lookup failed: member=%#v csrf=%q err=%v", member, csrf, err)
	}
	if _, err := service.SignIn(ctx, member.FullUsername, "2468"); err != nil {
		t.Fatalf("sign in: %v", err)
	}
	if _, err := service.SignIn(ctx, member.FullUsername, "0000"); !errors.Is(err, ErrUnauthenticated) {
		t.Fatalf("wrong PIN error = %v", err)
	}

	if err := service.SaveAttendance(ctx, member, "2026-07-14", 960, 1080); err != nil {
		t.Fatalf("save Tuesday: %v", err)
	}
	if err := service.SaveAttendance(ctx, member, "2026-07-15", 900, 1080); err != nil {
		t.Fatalf("save Wednesday: %v", err)
	}
	plans, err := service.Plans(ctx, member)
	if err != nil || len(plans) != 2 {
		t.Fatalf("plans=%#v err=%v", plans, err)
	}
	if err := service.SaveAttendance(ctx, member, "2026-07-14", 990, 1110); err != nil {
		t.Fatalf("replace Tuesday: %v", err)
	}
	plans, _ = service.Plans(ctx, member)
	if len(plans) != 2 || plans[0].StartMinute != 960 || plans[0].EndMinute != 1110 {
		t.Fatalf("overlapping addition was not merged: %#v", plans)
	}
	if err := service.SaveAttendanceIntervals(ctx, member, "2026-07-14", []AttendanceInterval{{StartMinute: 720, EndMinute: 840}, {StartMinute: 960, EndMinute: 1080}}); err != nil {
		t.Fatalf("save split Tuesday: %v", err)
	}
	plans, _ = service.Plans(ctx, member)
	if len(plans) != 3 || plans[0].StartMinute != 720 || plans[1].StartMinute != 960 || plans[1].EndMinute != 1110 {
		t.Fatalf("same-day split intervals were not saved: %#v", plans)
	}
}

func TestAttendanceRejectsUnavailableAndInvalidRanges(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	session, err := service.CreateAccount(ctx, "Range Tester", "1357", "visitor", true)
	if err != nil {
		t.Fatal(err)
	}
	member := session.Member
	for _, test := range []struct {
		date       string
		start, end int
	}{
		{"2026-07-14", 840, 900},
		{"2026-07-14", 960, 975},
		{"2026-07-28", 960, 1020},
	} {
		if err := service.SaveAttendance(ctx, member, test.date, test.start, test.end); err == nil {
			t.Fatalf("invalid plan accepted: %#v", test)
		}
	}
	if err := service.SaveAttendanceIntervals(ctx, member, "2026-07-14", []AttendanceInterval{{StartMinute: 960, EndMinute: 1080}, {StartMinute: 1020, EndMinute: 1140}}); err == nil {
		t.Fatal("overlapping split intervals were accepted")
	}
}

func TestPINHashCarriesVersionAndParameters(t *testing.T) {
	hash, err := hashPIN("2468")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(hash, "argon2id$v=19$m=65536,t=1,p=2$") {
		t.Fatalf("hash parameters missing: %q", hash)
	}
	if !verifyPIN(hash, "2468") || verifyPIN(hash, "8642") {
		t.Fatal("PIN verification result was incorrect")
	}
	for _, malformed := range []string{"", "argon2id$bad", "argon2id$v=19$m=1,t=1,p=1$bad$bad", hash + "$extra"} {
		if verifyPIN(malformed, "2468") {
			t.Fatalf("malformed hash accepted: %q", malformed)
		}
	}
}

func TestUsernameNormalizationAndRevokedSessionMutation(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	session, err := service.CreateAccount(ctx, "Case Player", "2468", "member", true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.SignIn(ctx, "  "+strings.ToUpper(session.Member.FullUsername)+"  ", "2468"); err != nil {
		t.Fatalf("normalized sign in failed: %v", err)
	}
	service.SignOut(ctx, session.Token)
	err = service.SaveAttendanceIntervalsForToken(ctx, session.Token, "2026-07-14", []AttendanceInterval{{StartMinute: 960, EndMinute: 1080}})
	if err == nil {
		t.Fatal("revoked session mutated attendance")
	}
	plans, err := service.Plans(ctx, session.Member)
	if err != nil || len(plans) != 0 {
		t.Fatalf("plans=%#v err=%v", plans, err)
	}
}

func TestProfileAndPINChangePreserveCurrentSessionAndRevokeOthers(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	current, err := service.CreateAccount(ctx, "Lifecycle Player", "2468", "member", true)
	if err != nil {
		t.Fatal(err)
	}
	other, err := service.SignIn(ctx, current.Member.FullUsername, "2468")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.UpdateProfile(ctx, current.Token, "Updated Player", "visitor", "2468"); err != nil {
		t.Fatalf("update profile: %v", err)
	}
	member, _, err := service.MemberForToken(ctx, current.Token)
	if err != nil || member.DisplayName != "Updated Player" || member.MemberStatus != "visitor" {
		t.Fatalf("updated member=%#v err=%v", member, err)
	}
	if err := service.ChangePIN(ctx, current.Token, "2468", "8642"); err != nil {
		t.Fatalf("change PIN: %v", err)
	}
	if _, _, err := service.MemberForToken(ctx, current.Token); err != nil {
		t.Fatalf("current session was revoked: %v", err)
	}
	if _, _, err := service.MemberForToken(ctx, other.Token); err == nil {
		t.Fatal("other session survived PIN change")
	}
	if _, err := service.SignIn(ctx, current.Member.FullUsername, "2468"); err == nil {
		t.Fatal("old PIN still authenticates")
	}
	if _, err := service.SignIn(ctx, current.Member.FullUsername, "8642"); err != nil {
		t.Fatalf("new PIN does not authenticate: %v", err)
	}
}

func TestTemporaryPINRequiresReplacement(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	created, err := service.CreateAccount(ctx, "Recovery Player", "2468", "member", true)
	if err != nil {
		t.Fatal(err)
	}
	temporaryHash, err := hashPIN("1357")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.store.SetTemporaryPIN(ctx, created.Member.ID, temporaryHash); err != nil {
		t.Fatal(err)
	}
	temporary, err := service.SignIn(ctx, created.Member.FullUsername, "1357")
	if err != nil || !temporary.Member.MustChangePIN {
		t.Fatalf("temporary session=%#v err=%v", temporary, err)
	}
	if err := service.SaveAttendanceIntervalsForToken(ctx, temporary.Token, "2026-07-14", []AttendanceInterval{{StartMinute: 960, EndMinute: 1080}}); err == nil {
		t.Fatal("temporary-PIN session changed attendance")
	}
	if err := service.ChangePIN(ctx, temporary.Token, "1357", "9753"); err != nil {
		t.Fatalf("replace temporary PIN: %v", err)
	}
	member, _, err := service.MemberForToken(ctx, temporary.Token)
	if err != nil || member.MustChangePIN {
		t.Fatalf("forced PIN state was not cleared: member=%#v err=%v", member, err)
	}
}

func TestSelfDeleteRemovesFuturePlansAndAnonymizesHistory(t *testing.T) {
	service := testService(t)
	ctx := context.Background()
	created, err := service.CreateAccount(ctx, "Delete Player", "2468", "member", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SaveAttendance(ctx, created.Member, "2026-07-14", 960, 1080); err != nil {
		t.Fatal(err)
	}
	if err := service.store.ReplaceAttendance(ctx, created.Member.ID, "2026-07-13", []sqlite.AttendancePlan{{Date: "2026-07-13", StartMinute: 960, EndMinute: 1080}}, time.Unix(1, 0)); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteAccount(ctx, created.Token, "2468", created.Member.FullUsername); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SignIn(ctx, created.Member.FullUsername, "2468"); err == nil {
		t.Fatal("deleted account still authenticates")
	}
	from, _ := domain.ParseCivilDate("2026-07-13")
	data, err := service.store.LoadAnonymousTimetable(ctx, from, from)
	if err != nil {
		t.Fatal(err)
	}
	if len(data.Attendance) != 1 || data.Attendance[0].AttendanceDate != "2026-07-13" {
		t.Fatalf("historical aggregate was not retained anonymously: %#v", data.Attendance)
	}
	future, err := service.store.PlansForAccount(ctx, created.Member.ID, "2026-07-14", "2026-07-27")
	if err != nil || len(future) != 0 {
		t.Fatalf("future plans=%#v err=%v", future, err)
	}
}
