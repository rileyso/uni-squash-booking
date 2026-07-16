// Package web provides the progressive HTML adapter.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/app"
)

//go:embed templates/*.html static/*
var assets embed.FS

type Server struct {
	app      *app.Service
	template *template.Template
	limits   *attemptLimiter
}

type indexData struct {
	Synthetic  bool
	Dashboard  app.Dashboard
	Member     app.Member
	CSRF       string
	Plans      []app.Plan
	Attendance *attendanceData
}

func New(application *app.Service) (*Server, error) {
	tmpl, err := template.New("").Funcs(template.FuncMap{"toggleTimeHref": toggleTimeHref, "hasPlanAt": hasPlanAt}).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{app: application, template: tmpl, limits: newAttemptLimiter()}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.index)
	if s.app.IdentityEnabled() {
		mux.HandleFunc("GET /sign-in", s.signInPage)
		mux.HandleFunc("POST /sign-in", s.signIn)
		mux.HandleFunc("GET /accounts/new", s.createAccountPage)
		mux.HandleFunc("POST /accounts", s.createAccount)
		mux.HandleFunc("GET /account-created", s.accountCreated)
		mux.HandleFunc("POST /sign-out", s.signOut)
		mux.HandleFunc("GET /attendance/new", s.attendancePage)
		mux.HandleFunc("POST /attendance/review", s.reviewAttendance)
		mux.HandleFunc("POST /attendance", s.saveAttendance)
		mux.HandleFunc("POST /attendance/remove", s.removeAttendance)
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	mux.HandleFunc("GET /readyz", s.ready)
	static, _ := fs.Sub(assets, "static")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))
	return securityHeaders(requestLog(mux))
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	data, err := s.indexPageData(r, r.URL.Query().Get("date"), r.URL.Query().Get("time"))
	if err != nil {
		http.Error(w, "Current attendance could not be loaded. Please try again.", http.StatusServiceUnavailable)
		return
	}
	data.Attendance = s.attendanceOverlayData(r, data.Member, data.CSRF)
	s.render(w, "index", data)
}

func (s *Server) indexPageData(r *http.Request, date, selectedTime string) (indexData, error) {
	dashboard, err := s.app.Dashboard(r.Context(), date, selectedTime)
	if err != nil {
		return indexData{}, err
	}
	member, csrf, _ := s.currentMember(r)
	plans, _ := s.app.Plans(r.Context(), member)
	data := indexData{Synthetic: s.app.Synthetic(), Dashboard: dashboard, Member: member, CSRF: csrf, Plans: plans}
	return data, nil
}

const sessionCookie = "squash_member"

type authPageData struct {
	indexData
	Create                         bool
	Error, IntentDate, IntentStart string
	IntentEnd                      string
	IntentTime                     string
}

type accountCreatedPageData struct {
	indexData
	Continue string
}

func (s *Server) signInPage(w http.ResponseWriter, r *http.Request) {
	s.renderAuth(w, r, authPageData{IntentDate: r.URL.Query().Get("intent_date"), IntentStart: r.URL.Query().Get("intent_start"), IntentEnd: r.URL.Query().Get("intent_end"), IntentTime: r.URL.Query().Get("intent_time")})
}
func (s *Server) createAccountPage(w http.ResponseWriter, r *http.Request) {
	s.renderAuth(w, r, authPageData{Create: true, IntentDate: r.URL.Query().Get("intent_date"), IntentStart: r.URL.Query().Get("intent_start"), IntentEnd: r.URL.Query().Get("intent_end"), IntentTime: r.URL.Query().Get("intent_time")})
}

func (s *Server) signIn(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", 400)
		return
	}
	username := r.FormValue("username")
	if !s.limits.loginAllowed(username, r.RemoteAddr) {
		w.Header().Set("Retry-After", "900")
		w.WriteHeader(http.StatusTooManyRequests)
		s.renderAuth(w, r, authPageData{Error: "Too many attempts. Try again later.", IntentDate: r.FormValue("intent_date"), IntentStart: r.FormValue("intent_start"), IntentEnd: r.FormValue("intent_end"), IntentTime: r.FormValue("intent_time")})
		return
	}
	session, err := s.app.SignIn(r.Context(), username, r.FormValue("pin"))
	if err != nil {
		s.limits.loginFailed(username, r.RemoteAddr)
		w.WriteHeader(http.StatusUnauthorized)
		s.renderAuth(w, r, authPageData{Error: "The username or PIN was not recognised.", IntentDate: r.FormValue("intent_date"), IntentStart: r.FormValue("intent_start"), IntentEnd: r.FormValue("intent_end"), IntentTime: r.FormValue("intent_time")})
		return
	}
	s.limits.loginSucceeded(username)
	s.setSession(w, session)
	http.Redirect(w, r, intentDestination(r.FormValue("intent_date"), r.FormValue("intent_start"), r.FormValue("intent_end"), r.FormValue("intent_time")), http.StatusSeeOther)
}

func (s *Server) accountCreated(w http.ResponseWriter, r *http.Request) {
	member, _, err := s.currentMember(r)
	if err != nil {
		http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
		return
	}
	data, err := s.indexPageData(r, r.URL.Query().Get("intent_date"), r.URL.Query().Get("intent_time"))
	if err != nil {
		http.Error(w, "Current attendance could not be loaded. Please try again.", http.StatusServiceUnavailable)
		return
	}
	data.Member = member
	s.render(w, "account-created", accountCreatedPageData{indexData: data, Continue: intentDestination(r.URL.Query().Get("intent_date"), r.URL.Query().Get("intent_start"), r.URL.Query().Get("intent_end"), r.URL.Query().Get("intent_time"))})
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", 400)
		return
	}
	if !s.limits.creationAllowed(r.RemoteAddr, r.UserAgent()) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
		s.renderAuth(w, r, authPageData{Create: true, Error: "Too many account attempts. Try again later.", IntentDate: r.FormValue("intent_date"), IntentStart: r.FormValue("intent_start"), IntentEnd: r.FormValue("intent_end"), IntentTime: r.FormValue("intent_time")})
		return
	}
	pin := r.FormValue("pin")
	if pin != r.FormValue("pin_confirm") {
		w.WriteHeader(422)
		s.renderAuth(w, r, authPageData{Create: true, Error: "The PINs do not match.", IntentDate: r.FormValue("intent_date"), IntentStart: r.FormValue("intent_start"), IntentEnd: r.FormValue("intent_end"), IntentTime: r.FormValue("intent_time")})
		return
	}
	session, err := s.app.CreateAccount(r.Context(), r.FormValue("display_name"), pin, r.FormValue("member_status"), r.FormValue("privacy_ack") != "")
	if err != nil {
		w.WriteHeader(422)
		s.renderAuth(w, r, authPageData{Create: true, Error: "Check the account details and privacy acknowledgement.", IntentDate: r.FormValue("intent_date"), IntentStart: r.FormValue("intent_start"), IntentEnd: r.FormValue("intent_end"), IntentTime: r.FormValue("intent_time")})
		return
	}
	s.setSession(w, session)
	q := url.Values{"intent_date": {r.FormValue("intent_date")}, "intent_start": {r.FormValue("intent_start")}, "intent_end": {r.FormValue("intent_end")}, "intent_time": {r.FormValue("intent_time")}}
	http.Redirect(w, r, "/account-created?"+q.Encode(), http.StatusSeeOther)
}

func (s *Server) signOut(w http.ResponseWriter, r *http.Request) {
	if m, csrf, err := s.currentMember(r); err == nil && m.ID > 0 && validCSRF(r, csrf) {
		if c, e := r.Cookie(sessionCookie); e == nil {
			s.app.SignOut(r.Context(), c.Value)
		}
	} else {
		http.Error(w, "Invalid form", http.StatusForbidden)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

type timeOption struct {
	Minute int
	Label  string
}

const (
	attendanceRangeStart = 10 * 60
	attendanceRangeEnd   = 22 * 60
	sliderMinDuration    = 60
)

type minuteRange struct {
	Start int
	End   int
}

type attendanceData struct {
	Synthetic         bool
	Member            app.Member
	CSRF, Error, Date string
	DateHeading       string
	Start, End        int
	MinMinute         int
	MaxMinute         int
	MinDuration       int
	OpenRanges        string
	Existing          bool
	Review            bool
	StartLabel        string
	EndLabel          string
	Duration          string
	Times             []timeOption
}

func (s *Server) attendancePage(w http.ResponseWriter, r *http.Request) {
	q := url.Values{"date": {r.URL.Query().Get("date")}, "add": {r.URL.Query().Get("start")}}
	if end := r.URL.Query().Get("end"); end != "" {
		q.Set("end", end)
	}
	if r.URL.Query().Get("review") == "1" {
		q.Set("review", "1")
	}
	http.Redirect(w, r, "/?"+q.Encode(), http.StatusSeeOther)
}

func (s *Server) attendanceOverlayData(r *http.Request, member app.Member, csrf string) *attendanceData {
	startText := r.URL.Query().Get("add")
	if startText == "" {
		return nil
	}
	date := r.URL.Query().Get("date")
	start, _ := strconv.Atoi(startText)
	if start%30 != 0 {
		start = 1020
	}
	end := start + 60
	if requestedEnd, err := strconv.Atoi(r.URL.Query().Get("end")); err == nil && requestedEnd > start && requestedEnd%30 == 0 {
		end = requestedEnd
	}
	var existing *app.Plan
	if member.ID > 0 {
		existing, _ = s.app.PlanForDate(r.Context(), member, date)
	}
	has := existing != nil
	if has && r.URL.Query().Get("review") != "1" {
		start, end = existing.StartMinute, existing.EndMinute
	}
	ranges := s.openPlayableRanges(r, date)
	start, end, selectedRange, rangeAvailable := clampSliderInterval(start, end, ranges)
	minMinute, maxMinute := attendanceRangeStart, attendanceRangeEnd
	openRanges := ""
	if rangeAvailable {
		minMinute, maxMinute = selectedRange.Start, selectedRange.End
		openRanges = rangeSpec([]minuteRange{selectedRange})
	}
	data := &attendanceData{
		Synthetic:   s.app.Synthetic(),
		Member:      member,
		CSRF:        csrf,
		Date:        date,
		DateHeading: dateHeading(date),
		Start:       start,
		End:         end,
		MinMinute:   minMinute,
		MaxMinute:   maxMinute,
		MinDuration: sliderMinDuration,
		OpenRanges:  openRanges,
		Existing:    has,
		Times:       timeOptions(),
	}
	data.StartLabel = fmtTime(start/60, start%60)
	data.EndLabel = fmtTime(end/60, end%60)
	data.Duration = formatDuration(end - start)
	if r.URL.Query().Get("attendance_error") != "" {
		data.Error = "That interval is not continuously open for play. Choose another range."
	}
	if r.URL.Query().Get("review") == "1" {
		if err := s.app.ValidateAttendance(r.Context(), data.Date, start, end); err == nil {
			data.Review = true
		} else {
			data.Error = "That interval is not continuously open for play. Choose another range."
		}
	}
	return data
}

func (s *Server) openPlayableRanges(r *http.Request, date string) []minuteRange {
	var ranges []minuteRange
	current := minuteRange{Start: -1}
	for minute := attendanceRangeStart; minute < attendanceRangeEnd; minute += 30 {
		open := s.app.ValidateAttendance(r.Context(), date, minute, minute+30) == nil
		if open && current.Start == -1 {
			current.Start = minute
		}
		if (!open || minute+30 == attendanceRangeEnd) && current.Start != -1 {
			end := minute
			if open {
				end = minute + 30
			}
			if end-current.Start >= sliderMinDuration {
				ranges = append(ranges, minuteRange{Start: current.Start, End: end})
			}
			current = minuteRange{Start: -1}
		}
	}
	return ranges
}

func clampSliderInterval(start, end int, ranges []minuteRange) (int, int, minuteRange, bool) {
	if len(ranges) == 0 {
		return attendanceRangeStart, attendanceRangeStart + sliderMinDuration, minuteRange{Start: attendanceRangeStart, End: attendanceRangeEnd}, false
	}
	if start < attendanceRangeStart || start > attendanceRangeEnd || start%30 != 0 {
		start = ranges[0].Start
	}
	if end <= start || end > attendanceRangeEnd || end%30 != 0 {
		end = start + sliderMinDuration
	}
	for _, candidate := range ranges {
		if start >= candidate.Start && start+sliderMinDuration <= candidate.End {
			if end > candidate.End {
				end = candidate.End
			}
			if end-start < sliderMinDuration {
				end = start + sliderMinDuration
			}
			return start, end, candidate, true
		}
	}
	for _, candidate := range ranges {
		if start < candidate.End {
			return candidate.Start, candidate.Start + sliderMinDuration, candidate, true
		}
	}
	last := ranges[len(ranges)-1]
	return last.End - sliderMinDuration, last.End, last, true
}

func rangeSpec(ranges []minuteRange) string {
	parts := make([]string, 0, len(ranges))
	for _, item := range ranges {
		parts = append(parts, fmt.Sprintf("%d-%d", item.Start, item.End))
	}
	return strings.Join(parts, ",")
}

func (s *Server) reviewAttendance(w http.ResponseWriter, r *http.Request) {
	member, csrf, _ := s.currentMember(r)
	if err := r.ParseForm(); err != nil || (member.ID > 0 && !validCSRF(r, csrf)) {
		http.Error(w, "Invalid form", http.StatusForbidden)
		return
	}
	start, _ := strconv.Atoi(r.FormValue("start"))
	end, _ := strconv.Atoi(r.FormValue("end"))
	data := attendanceData{Synthetic: s.app.Synthetic(), Member: member, CSRF: csrf, Date: r.FormValue("date"), Start: start, End: end, Times: timeOptions()}
	if err := s.app.ValidateAttendance(r.Context(), data.Date, start, end); err != nil {
		q := url.Values{"date": {data.Date}, "add": {strconv.Itoa(start)}, "end": {strconv.Itoa(end)}, "attendance_error": {"1"}}
		http.Redirect(w, r, "/?"+q.Encode(), http.StatusSeeOther)
		return
	}
	q := url.Values{"date": {data.Date}, "add": {strconv.Itoa(start)}, "end": {strconv.Itoa(end)}, "review": {"1"}}
	http.Redirect(w, r, "/?"+q.Encode(), http.StatusSeeOther)
}

func (s *Server) saveAttendance(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", 403)
		return
	}
	intervals := attendanceIntervalsFromForm(r)
	start, end := 0, 0
	if len(intervals) > 0 {
		start, end = intervals[0].StartMinute, intervals[len(intervals)-1].EndMinute
	}
	member, csrf, err := s.currentMember(r)
	if err != nil {
		q := url.Values{"intent_date": {r.FormValue("date")}, "intent_start": {strconv.Itoa(start)}, "intent_end": {strconv.Itoa(end)}}
		if _, fromSelection := r.Form["range_start"]; fromSelection {
			q.Set("intent_time", selectedMinuteQueryFromIntervals(intervals))
		}
		http.Redirect(w, r, "/sign-in?"+q.Encode(), http.StatusSeeOther)
		return
	}
	if !validCSRF(r, csrf) {
		http.Error(w, "Invalid form", 403)
		return
	}
	if err := s.app.SaveAttendanceIntervals(r.Context(), member, r.FormValue("date"), intervals); err != nil {
		q := url.Values{"date": {r.FormValue("date")}, "add": {strconv.Itoa(start)}, "end": {strconv.Itoa(end)}, "attendance_error": {"1"}}
		http.Redirect(w, r, "/?"+q.Encode(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/?date="+url.QueryEscape(r.FormValue("date")), http.StatusSeeOther)
}

func attendanceIntervalsFromForm(r *http.Request) []app.AttendanceInterval {
	starts, ends := r.Form["range_start"], r.Form["range_end"]
	if len(starts) > 0 && len(starts) == len(ends) {
		intervals := make([]app.AttendanceInterval, 0, len(starts))
		for index := range starts {
			start, startErr := strconv.Atoi(starts[index])
			end, endErr := strconv.Atoi(ends[index])
			if startErr != nil || endErr != nil {
				return nil
			}
			intervals = append(intervals, app.AttendanceInterval{StartMinute: start, EndMinute: end})
		}
		return intervals
	}
	start, startErr := strconv.Atoi(r.FormValue("start"))
	end, endErr := strconv.Atoi(r.FormValue("end"))
	if startErr != nil || endErr != nil {
		return nil
	}
	return []app.AttendanceInterval{{StartMinute: start, EndMinute: end}}
}

func selectedMinuteQueryFromIntervals(intervals []app.AttendanceInterval) string {
	var minutes []int
	seen := map[int]bool{}
	for _, interval := range intervals {
		for minute := interval.StartMinute; minute < interval.EndMinute; minute += 60 {
			if seen[minute] {
				continue
			}
			seen[minute] = true
			minutes = append(minutes, minute)
		}
	}
	sort.Ints(minutes)
	parts := make([]string, 0, len(minutes))
	for _, minute := range minutes {
		parts = append(parts, strconv.Itoa(minute))
	}
	return strings.Join(parts, ",")
}
func (s *Server) removeAttendance(w http.ResponseWriter, r *http.Request) {
	member, csrf, err := s.currentMember(r)
	if err != nil {
		http.Error(w, "Sign in required", 401)
		return
	}
	if err := r.ParseForm(); err != nil || !validCSRF(r, csrf) {
		http.Error(w, "Invalid form", 403)
		return
	}
	_ = s.app.RemoveAttendance(r.Context(), member, r.FormValue("date"))
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) currentMember(r *http.Request) (app.Member, string, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return app.Member{}, "", err
	}
	return s.app.MemberForToken(r.Context(), cookie.Value)
}
func (s *Server) setSession(w http.ResponseWriter, session app.Session) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: session.Token, Path: "/", Expires: session.Expires, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.app.SecureCookies()})
}
func (s *Server) render(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.template.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "The page is temporarily unavailable.", 500)
	}
}

func (s *Server) renderAuth(w http.ResponseWriter, r *http.Request, data authPageData) {
	index, err := s.indexPageData(r, data.IntentDate, data.IntentTime)
	if err != nil {
		http.Error(w, "Current attendance could not be loaded. Please try again.", http.StatusServiceUnavailable)
		return
	}
	data.indexData = index
	s.render(w, "auth", data)
}

func validCSRF(r *http.Request, want string) bool {
	return want != "" && strings.EqualFold(r.FormValue("csrf"), want)
}

func toggleTimeHref(date, selected string, minute int) string {
	seen := map[int]bool{}
	var minutes []int
	removed := false
	for _, part := range strings.Split(selected, ",") {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || seen[value] {
			continue
		}
		seen[value] = true
		if value == minute {
			removed = true
			continue
		}
		minutes = append(minutes, value)
	}
	if !removed {
		minutes = append(minutes, minute)
	}
	q := url.Values{"date": {date}}
	if len(minutes) > 0 {
		parts := make([]string, 0, len(minutes))
		for _, value := range minutes {
			parts = append(parts, strconv.Itoa(value))
		}
		q.Set("time", strings.Join(parts, ","))
	}
	return "/?" + q.Encode()
}

func hasPlanAt(plans []app.Plan, date string, minute int) bool {
	for _, plan := range plans {
		if plan.Date == date && minute >= plan.StartMinute && minute < plan.EndMinute {
			return true
		}
	}
	return false
}

func intentDestination(date, start, end, selectedTime string) string {
	if date == "" {
		return "/"
	}
	if selectedTime != "" {
		q := url.Values{"date": {date}, "time": {selectedTime}}
		return "/?" + q.Encode()
	}
	q := url.Values{"date": {date}, "add": {start}}
	if end != "" {
		q.Set("end", end)
		q.Set("review", "1")
	}
	return "/?" + q.Encode()
}
func timeOptions() []timeOption {
	var result []timeOption
	for minute := 0; minute <= 1440; minute += 30 {
		h := minute / 60
		m := minute % 60
		label := fmtTime(h, m)
		result = append(result, timeOption{minute, label})
	}
	return result
}
func fmtTime(hour, minute int) string {
	if hour == 24 {
		return "12:00 am"
	}
	return time.Date(2000, 1, 1, hour, minute, 0, 0, time.UTC).Format("3:04 pm")
}

func formatDuration(minutes int) string {
	if minutes == 30 {
		return "30 minutes"
	}
	if minutes%60 == 0 {
		return strconv.Itoa(minutes/60) + " hours"
	}
	return strconv.Itoa(minutes/60) + "½ hours"
}

func dateHeading(date string) string {
	parsed, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return parsed.Format("Mon 2 January")
}

func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := contextWithTimeout(r, 2*time.Second)
	defer cancel()
	if err := s.app.Ready(ctx); err != nil {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
