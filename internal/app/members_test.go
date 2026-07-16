package app

import (
	"context"
	"errors"
	"testing"
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
	if len(plans) != 2 || plans[0].StartMinute != 990 {
		t.Fatalf("same-day plan was not replaced: %#v", plans)
	}
	if err := service.SaveAttendanceIntervals(ctx, member, "2026-07-14", []AttendanceInterval{{StartMinute: 720, EndMinute: 840}, {StartMinute: 960, EndMinute: 1080}}); err != nil {
		t.Fatalf("save split Tuesday: %v", err)
	}
	plans, _ = service.Plans(ctx, member)
	if len(plans) != 3 || plans[0].StartMinute != 720 || plans[1].StartMinute != 960 {
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
