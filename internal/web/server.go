// Package web provides the progressive HTML adapter.
package web

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/app"
	"github.com/rileyso/uni-squash-booking/internal/sqlite"
)

//go:embed templates/*.html static/*
var assets embed.FS

type Server struct {
	app         *app.Service
	template    *template.Template
	limits      *attemptLimiter
	adminLimits *attemptLimiter
	deviceKey   []byte
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
	tmpl, err := template.New("").Funcs(template.FuncMap{"toggleTimeHref": toggleTimeHref, "hasPlanAt": hasPlanAt, "scheduleTime": scheduleTime, "weekdayLabel": weekdayLabel, "scheduleKind": scheduleKind, "days": func() []int { return []int{1, 2, 3, 4, 5, 6, 7} }}).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	deviceKey := make([]byte, 32)
	if configured := application.DeviceCookieSecret(); configured != "" {
		deviceKey = []byte(configured)
	} else if _, err := rand.Read(deviceKey); err != nil {
		return nil, fmt.Errorf("generate anonymous device key: %w", err)
	}
	return &Server{app: application, template: tmpl, limits: newAttemptLimiter(), adminLimits: newAttemptLimiter(), deviceKey: deviceKey}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.index)
	mux.HandleFunc("GET /attendance", func(w http.ResponseWriter, r *http.Request) {
		destination := "/"
		if r.URL.RawQuery != "" {
			destination += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, destination, http.StatusTemporaryRedirect)
	})
	if s.app.AdminEnabled() {
		mux.HandleFunc("GET /admin/sign-in", s.adminSignInPage)
		mux.HandleFunc("POST /admin/sign-in", s.adminSignIn)
		mux.HandleFunc("POST /admin/sign-out", s.adminSignOut)
		mux.HandleFunc("GET /admin/accounts", s.adminAccounts)
		mux.HandleFunc("POST /admin/accounts/{id}/reset-pin", s.adminResetPIN)
		mux.HandleFunc("GET /admin/schedule", s.adminSchedule)
		mux.HandleFunc("POST /admin/schedule/weekly", s.adminCreateWeeklySchedule)
		mux.HandleFunc("POST /admin/schedule/dated", s.adminCreateDatedSchedule)
		mux.HandleFunc("POST /admin/schedule/social", s.adminCreateSocialSession)
		mux.HandleFunc("POST /admin/schedule/{type}/{id}/delete", s.adminDeleteScheduleEntry)
	}
	if s.app.NamedAttendanceEnabled() {
		mux.HandleFunc("GET /attendance/participants", s.participantNames)
	}
	if s.app.IdentityEnabled() {
		mux.HandleFunc("GET /sign-in", s.signInPage)
		mux.HandleFunc("POST /sign-in", s.signIn)
		mux.HandleFunc("GET /accounts/new", s.createAccountPage)
		mux.HandleFunc("POST /accounts", s.createAccount)
		mux.HandleFunc("GET /account-created", s.accountCreated)
		mux.HandleFunc("POST /sign-out", s.signOut)
		mux.HandleFunc("GET /account/profile", s.profilePage)
		mux.HandleFunc("POST /account/profile", s.updateProfile)
		mux.HandleFunc("GET /account/pin", s.pinPage)
		mux.HandleFunc("POST /account/pin", s.changePIN)
		if s.app.SelfDeleteEnabled() {
			mux.HandleFunc("GET /account/delete", s.deleteAccountPage)
			mux.HandleFunc("POST /account/delete", s.deleteAccount)
		}
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

func (s *Server) participantNames(w http.ResponseWriter, r *http.Request) {
	start, startErr := strconv.Atoi(r.URL.Query().Get("start"))
	end, endErr := strconv.Atoi(r.URL.Query().Get("end"))
	if startErr != nil || endErr != nil {
		http.Error(w, "Invalid interval", http.StatusBadRequest)
		return
	}
	names, err := s.app.ParticipantNames(r.Context(), r.URL.Query().Get("date"), start, end)
	if err != nil {
		http.Error(w, "Participant names are unavailable", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = json.NewEncoder(w).Encode(struct {
		Names []string `json:"names"`
	}{Names: names})
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if member, _, err := s.currentMember(r); err == nil && member.MustChangePIN {
		http.Redirect(w, r, "/account/pin", http.StatusSeeOther)
		return
	}
	data, err := s.indexPageData(r, r.URL.Query().Get("date"), r.URL.Query().Get("time"))
	if err != nil {
		http.Error(w, "Current attendance could not be loaded. Please try again.", http.StatusServiceUnavailable)
		return
	}
	data.Attendance = s.attendanceOverlayData(r, data.Member, data.CSRF)
	w.Header().Add("Vary", "HX-Request")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Cache-Control", "private, no-store")
		s.render(w, "timetable-interaction", data)
		return
	}
	s.render(w, "index", data)
}

type accountPageData struct {
	Synthetic  bool
	Member     app.Member
	CSRF       string
	Error      string
	Success    string
	PIN        bool
	Delete     bool
	SelfDelete bool
}

func (s *Server) profilePage(w http.ResponseWriter, r *http.Request) {
	member, csrf, err := s.currentMember(r)
	if err != nil {
		http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
		return
	}
	if member.MustChangePIN {
		http.Redirect(w, r, "/account/pin", http.StatusSeeOther)
		return
	}
	s.render(w, "account", accountPageData{Synthetic: s.app.Synthetic(), Member: member, CSRF: csrf, SelfDelete: s.app.SelfDeleteEnabled()})
}

func (s *Server) updateProfile(w http.ResponseWriter, r *http.Request) {
	member, csrf, err := s.currentMember(r)
	if err != nil || member.MustChangePIN || r.ParseForm() != nil || !validCSRF(r, csrf) {
		http.Error(w, "Invalid form", http.StatusForbidden)
		return
	}
	cookie, _ := r.Cookie(sessionCookie)
	if err := s.app.UpdateProfile(r.Context(), cookie.Value, r.FormValue("display_name"), r.FormValue("member_status"), r.FormValue("current_pin")); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.render(w, "account", accountPageData{Synthetic: s.app.Synthetic(), Member: member, CSRF: csrf, SelfDelete: s.app.SelfDeleteEnabled(), Error: "The profile could not be changed. Check your current PIN and details."})
		return
	}
	updated, updatedCSRF, _ := s.currentMember(r)
	s.render(w, "account", accountPageData{Synthetic: s.app.Synthetic(), Member: updated, CSRF: updatedCSRF, SelfDelete: s.app.SelfDeleteEnabled(), Success: "Profile updated."})
}

func (s *Server) deleteAccountPage(w http.ResponseWriter, r *http.Request) {
	member, csrf, err := s.currentMember(r)
	if err != nil || member.MustChangePIN {
		http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
		return
	}
	s.render(w, "account", accountPageData{Synthetic: s.app.Synthetic(), Member: member, CSRF: csrf, Delete: true, SelfDelete: true})
}

func (s *Server) deleteAccount(w http.ResponseWriter, r *http.Request) {
	member, csrf, err := s.currentMember(r)
	if err != nil || member.MustChangePIN || r.ParseForm() != nil || !validCSRF(r, csrf) {
		http.Error(w, "Invalid form", http.StatusForbidden)
		return
	}
	cookie, _ := r.Cookie(sessionCookie)
	if err := s.app.DeleteAccount(r.Context(), cookie.Value, r.FormValue("current_pin"), r.FormValue("username")); err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.render(w, "account", accountPageData{Synthetic: s.app.Synthetic(), Member: member, CSRF: csrf, Delete: true, SelfDelete: true, Error: "The account could not be deleted. Enter your exact username and current PIN."})
		return
	}
	log.Printf("security_event=member_account_deleted member_id=%d", member.ID)
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) pinPage(w http.ResponseWriter, r *http.Request) {
	member, csrf, err := s.currentMember(r)
	if err != nil {
		http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
		return
	}
	s.render(w, "account", accountPageData{Synthetic: s.app.Synthetic(), Member: member, CSRF: csrf, PIN: true})
}

func (s *Server) changePIN(w http.ResponseWriter, r *http.Request) {
	member, csrf, err := s.currentMember(r)
	if err != nil || r.ParseForm() != nil || !validCSRF(r, csrf) {
		http.Error(w, "Invalid form", http.StatusForbidden)
		return
	}
	data := accountPageData{Synthetic: s.app.Synthetic(), Member: member, CSRF: csrf, PIN: true}
	if r.FormValue("new_pin") != r.FormValue("new_pin_confirm") {
		data.Error = "The new PINs do not match."
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.render(w, "account", data)
		return
	}
	cookie, _ := r.Cookie(sessionCookie)
	if err := s.app.ChangePIN(r.Context(), cookie.Value, r.FormValue("current_pin"), r.FormValue("new_pin")); err != nil {
		data.Error = "The PIN could not be changed. Check the current PIN and choose a different four-digit PIN."
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.render(w, "account", data)
		return
	}
	log.Printf("security_event=member_pin_changed member_id=%d other_sessions_revoked=true", member.ID)
	data.Member.MustChangePIN = false
	data.Success = "PIN changed. Other signed-in devices have been signed out."
	s.render(w, "account", data)
}

func (s *Server) indexPageData(r *http.Request, date, selectedTime string) (indexData, error) {
	dashboard, err := s.app.Dashboard(r.Context(), date, selectedTime)
	if err != nil {
		return indexData{}, err
	}
	member, csrf, _ := s.currentMember(r)
	plans, _ := s.app.Plans(r.Context(), member)
	data := indexData{Synthetic: s.app.Synthetic(), Dashboard: dashboard, Member: member, CSRF: csrf, Plans: plans}
	removePlannedSelections(&data)
	mergeSelectionSummary(&data)
	return data, nil
}

func removePlannedSelections(data *indexData) {
	if data.Member.ID == 0 || !data.Dashboard.HasSelectedInterval {
		return
	}
	var minutes []int
	for _, part := range strings.Split(data.Dashboard.SelectedTimeQuery, ",") {
		minute, err := strconv.Atoi(part)
		if err == nil && !hasPlanAt(data.Plans, data.Dashboard.SelectedDate, minute) {
			minutes = append(minutes, minute)
		}
	}
	selected := make(map[int]bool, len(minutes))
	for _, minute := range minutes {
		selected[minute] = true
	}
	for index := range data.Dashboard.DesktopSlots {
		data.Dashboard.DesktopSlots[index].Selected = selected[data.Dashboard.DesktopSlots[index].StartMinute]
	}
	for dayIndex := range data.Dashboard.Days {
		for slotIndex := range data.Dashboard.Days[dayIndex].Slots {
			data.Dashboard.Days[dayIndex].Slots[slotIndex].Selected = data.Dashboard.Days[dayIndex].Date == data.Dashboard.SelectedDate && selected[data.Dashboard.Days[dayIndex].Slots[slotIndex].StartMinute]
		}
	}
	if len(minutes) == 0 {
		data.Dashboard.HasSelectedInterval = false
		data.Dashboard.Detail = nil
		data.Dashboard.SelectedTimeQuery = ""
		data.Dashboard.SelectedRanges = nil
		data.Dashboard.SummaryRanges = nil
		data.Dashboard.CanConfirmSelection = false
		return
	}
	sort.Ints(minutes)
	data.Dashboard.SelectedTimeQuery = selectedMinuteQuery(minutes)
	var ranges []app.SelectedRange
	start, end := minutes[0], minutes[0]+60
	for _, minute := range minutes[1:] {
		if minute == end {
			end += 60
			continue
		}
		ranges = append(ranges, webSelectedRange(start, end))
		start, end = minute, minute+60
	}
	ranges = append(ranges, webSelectedRange(start, end))
	data.Dashboard.SelectedRanges = ranges
	data.Dashboard.SummaryRanges = ranges
}

func selectedMinuteQuery(minutes []int) string {
	parts := make([]string, len(minutes))
	for index, minute := range minutes {
		parts[index] = strconv.Itoa(minute)
	}
	return strings.Join(parts, ",")
}

func webSelectedRange(start, end int) app.SelectedRange {
	return app.SelectedRange{StartMinute: start, EndMinute: end, StartLabel: fmtTime(start/60, start%60), EndLabel: fmtTime(end/60, end%60), Duration: formatDuration(end - start)}
}

func mergeSelectionSummary(data *indexData) {
	if !data.Dashboard.HasSelectedInterval || data.Member.ID == 0 {
		return
	}
	intervals := append([]app.SelectedRange(nil), data.Dashboard.SelectedRanges...)
	for _, plan := range data.Plans {
		if plan.Date == data.Dashboard.SelectedDate {
			intervals = append(intervals, app.SelectedRange{StartMinute: plan.StartMinute, EndMinute: plan.EndMinute})
		}
	}
	if len(intervals) == len(data.Dashboard.SelectedRanges) {
		return
	}
	sort.Slice(intervals, func(i, j int) bool { return intervals[i].StartMinute < intervals[j].StartMinute })
	merged := make([]app.SelectedRange, 0, len(intervals))
	for _, interval := range intervals {
		if len(merged) == 0 || interval.StartMinute > merged[len(merged)-1].EndMinute {
			merged = append(merged, interval)
			continue
		}
		if interval.EndMinute > merged[len(merged)-1].EndMinute {
			merged[len(merged)-1].EndMinute = interval.EndMinute
		}
	}
	total := 0
	for index := range merged {
		merged[index].StartLabel = fmtTime(merged[index].StartMinute/60, merged[index].StartMinute%60)
		merged[index].EndLabel = fmtTime(merged[index].EndMinute/60, merged[index].EndMinute%60)
		merged[index].Duration = formatDuration(merged[index].EndMinute - merged[index].StartMinute)
		total += merged[index].EndMinute - merged[index].StartMinute
	}
	data.Dashboard.SummaryRanges = merged
	data.Dashboard.SummaryTotal = formatDuration(total)
}

const sessionCookie = "squash_member"
const adminSessionCookie = "squash_admin"

type adminPageData struct {
	Synthetic    bool
	SignedIn     bool
	CSRF         string
	Query        string
	Error        string
	Results      []sqlite.AdminAccountResult
	TemporaryPIN string
}

type adminSchedulePageData struct {
	Synthetic bool
	CSRF      string
	Error     string
	Entries   []sqlite.AdminScheduleEntry
}

func scheduleTime(minute int) string {
	return fmt.Sprintf("%02d:%02d", minute/60, minute%60)
}

func weekdayLabel(day int) string {
	labels := []string{"", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}
	if day < 1 || day > 7 {
		return "Dated"
	}
	return labels[day]
}

func scheduleKind(kind string) string {
	labels := map[string]string{"open": "Open for play", "lights_off": "Lights off", "competition": "Competition", "coaching": "Coaching", "closure": "Other closure", "social": "Social session"}
	return labels[kind]
}

func (s *Server) adminSignInPage(w http.ResponseWriter, r *http.Request) {
	if _, err := s.currentAdmin(r); err == nil {
		http.Redirect(w, r, "/admin/accounts", http.StatusSeeOther)
		return
	}
	s.render(w, "admin", adminPageData{Synthetic: s.app.Synthetic()})
}

func (s *Server) adminSignIn(w http.ResponseWriter, r *http.Request) {
	if r.ParseForm() != nil {
		http.Error(w, "Invalid form", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	if !s.adminLimits.loginAllowed(username, r.RemoteAddr) {
		log.Printf("security_event=admin_login_throttled source=%s", clientHost(r.RemoteAddr))
		w.WriteHeader(http.StatusTooManyRequests)
		s.render(w, "admin", adminPageData{Synthetic: s.app.Synthetic(), Error: "Too many attempts. Try again later."})
		return
	}
	session, err := s.app.AdminSignIn(r.Context(), username, r.FormValue("password"))
	if err != nil {
		s.adminLimits.loginFailed(username, r.RemoteAddr)
		log.Printf("security_event=admin_login_failed source=%s", clientHost(r.RemoteAddr))
		w.WriteHeader(http.StatusUnauthorized)
		s.render(w, "admin", adminPageData{Synthetic: s.app.Synthetic(), Error: "The administrator credentials were not recognised."})
		return
	}
	s.adminLimits.loginSucceeded(username)
	log.Printf("security_event=admin_login_succeeded")
	http.SetCookie(w, &http.Cookie{Name: adminSessionCookie, Value: session.Token, Path: "/admin", Expires: session.Expires, HttpOnly: true, Secure: s.app.SecureCookies(), SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/admin/accounts", http.StatusSeeOther)
}

func (s *Server) currentAdmin(r *http.Request) (string, error) {
	cookie, err := r.Cookie(adminSessionCookie)
	if err != nil {
		return "", err
	}
	return s.app.AdminForToken(r.Context(), cookie.Value)
}

func (s *Server) adminSignOut(w http.ResponseWriter, r *http.Request) {
	csrf, err := s.currentAdmin(r)
	if err != nil || r.ParseForm() != nil || !validCSRF(r, csrf) {
		http.Error(w, "Invalid form", http.StatusForbidden)
		return
	}
	if cookie, err := r.Cookie(adminSessionCookie); err == nil {
		s.app.AdminSignOut(r.Context(), cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: adminSessionCookie, Path: "/admin", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/admin/sign-in", http.StatusSeeOther)
}

func (s *Server) adminAccounts(w http.ResponseWriter, r *http.Request) {
	csrf, err := s.currentAdmin(r)
	if err != nil {
		http.Redirect(w, r, "/admin/sign-in", http.StatusSeeOther)
		return
	}
	data := adminPageData{Synthetic: s.app.Synthetic(), SignedIn: true, CSRF: csrf, Query: r.URL.Query().Get("q")}
	if data.Query != "" {
		data.Results, err = s.app.SearchAccounts(r.Context(), data.Query)
		if err != nil {
			data.Error = "The account search could not be completed."
		}
	}
	s.render(w, "admin", data)
}

func (s *Server) adminResetPIN(w http.ResponseWriter, r *http.Request) {
	csrf, err := s.currentAdmin(r)
	if err != nil || r.ParseForm() != nil || !validCSRF(r, csrf) {
		http.Error(w, "Invalid form", http.StatusForbidden)
		return
	}
	accountID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "Invalid account", http.StatusBadRequest)
		return
	}
	temporaryPIN, err := s.app.ResetMemberPIN(r.Context(), accountID, r.FormValue("identity_check_attested") == "yes")
	data := adminPageData{Synthetic: s.app.Synthetic(), SignedIn: true, CSRF: csrf, Query: r.FormValue("q")}
	if err != nil {
		w.WriteHeader(http.StatusUnprocessableEntity)
		data.Error = "PIN reset requires the approved identity-check attestation and an exact account selection."
	} else {
		data.TemporaryPIN = temporaryPIN
		log.Printf("security_event=member_pin_reset account_id=%d sessions_revoked=true", accountID)
	}
	if data.Query != "" {
		data.Results, _ = s.app.SearchAccounts(r.Context(), data.Query)
	}
	s.render(w, "admin", data)
}

func (s *Server) adminSchedule(w http.ResponseWriter, r *http.Request) {
	csrf, err := s.currentAdmin(r)
	if err != nil {
		http.Redirect(w, r, "/admin/sign-in", http.StatusSeeOther)
		return
	}
	entries, err := s.app.AdminSchedule(r.Context())
	data := adminSchedulePageData{Synthetic: s.app.Synthetic(), CSRF: csrf, Entries: entries}
	if err != nil {
		data.Error = "The schedule could not be loaded."
	}
	s.render(w, "admin-schedule", data)
}

func (s *Server) parseAdminScheduleForm(w http.ResponseWriter, r *http.Request) (string, bool) {
	csrf, err := s.currentAdmin(r)
	if err != nil || r.ParseForm() != nil || !validCSRF(r, csrf) {
		http.Error(w, "Invalid form", http.StatusForbidden)
		return "", false
	}
	return csrf, true
}

func formInt(r *http.Request, name string) (int, error) {
	return strconv.Atoi(r.FormValue(name))
}

func (s *Server) finishAdminScheduleMutation(w http.ResponseWriter, r *http.Request, err error) {
	if err != nil {
		http.Error(w, "Use valid dates and 30-minute times, with the end after the start.", http.StatusUnprocessableEntity)
		return
	}
	http.Redirect(w, r, "/admin/schedule", http.StatusSeeOther)
}

func (s *Server) adminCreateWeeklySchedule(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.parseAdminScheduleForm(w, r); !ok {
		return
	}
	court, e1 := formInt(r, "court")
	weekday, e2 := formInt(r, "weekday")
	start, e3 := formInt(r, "start")
	end, e4 := formInt(r, "end")
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil {
		s.finishAdminScheduleMutation(w, r, app.ErrInvalidInput)
		return
	}
	s.finishAdminScheduleMutation(w, r, s.app.CreateWeeklySchedule(r.Context(), court, r.FormValue("kind"), r.FormValue("title"), weekday, start, end, r.FormValue("effective")))
}

func (s *Server) adminCreateDatedSchedule(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.parseAdminScheduleForm(w, r); !ok {
		return
	}
	court, e1 := formInt(r, "court")
	start, e2 := formInt(r, "start")
	end, e3 := formInt(r, "end")
	if e1 != nil || e2 != nil || e3 != nil {
		s.finishAdminScheduleMutation(w, r, app.ErrInvalidInput)
		return
	}
	s.finishAdminScheduleMutation(w, r, s.app.CreateDatedSchedule(r.Context(), court, r.FormValue("kind"), r.FormValue("title"), r.FormValue("date"), start, end))
}

func (s *Server) adminCreateSocialSession(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.parseAdminScheduleForm(w, r); !ok {
		return
	}
	weekday, e1 := formInt(r, "weekday")
	start, e2 := formInt(r, "start")
	end, e3 := formInt(r, "end")
	if e1 != nil || e2 != nil || e3 != nil {
		s.finishAdminScheduleMutation(w, r, app.ErrInvalidInput)
		return
	}
	s.finishAdminScheduleMutation(w, r, s.app.CreateSocialSession(r.Context(), weekday, start, end, r.FormValue("effective")))
}

func (s *Server) adminDeleteScheduleEntry(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.parseAdminScheduleForm(w, r); !ok {
		return
	}
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err == nil {
		err = s.app.DeleteScheduleEntry(r.Context(), r.PathValue("type"), id)
	}
	s.finishAdminScheduleMutation(w, r, err)
}

type authPageData struct {
	indexData
	Create                         bool
	Error, IntentDate, IntentStart string
	IntentEnd                      string
	IntentTime                     string
	Username                       string
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
		log.Printf("security_event=member_login_throttled source=%s", clientHost(r.RemoteAddr))
		w.Header().Set("Retry-After", "900")
		w.WriteHeader(http.StatusTooManyRequests)
		s.renderAuth(w, r, authPageData{Error: "Too many attempts. Try again later.", Username: username, IntentDate: r.FormValue("intent_date"), IntentStart: r.FormValue("intent_start"), IntentEnd: r.FormValue("intent_end"), IntentTime: r.FormValue("intent_time")})
		return
	}
	session, err := s.app.SignIn(r.Context(), username, r.FormValue("pin"))
	if err != nil {
		s.limits.loginFailed(username, r.RemoteAddr)
		log.Printf("security_event=member_login_failed source=%s", clientHost(r.RemoteAddr))
		w.WriteHeader(http.StatusUnauthorized)
		s.renderAuth(w, r, authPageData{Error: "The username or PIN was not recognised.", Username: username, IntentDate: r.FormValue("intent_date"), IntentStart: r.FormValue("intent_start"), IntentEnd: r.FormValue("intent_end"), IntentTime: r.FormValue("intent_time")})
		return
	}
	s.limits.loginSucceeded(username)
	log.Printf("security_event=member_login_succeeded member_id=%d", session.Member.ID)
	s.setSession(w, session)
	if session.Member.MustChangePIN {
		if r.Header.Get("HX-Request") == "true" {
			w.Header().Set("HX-Redirect", "/account/pin")
			return
		}
		http.Redirect(w, r, "/account/pin", http.StatusSeeOther)
		return
	}
	destination := intentDestination(r.FormValue("intent_date"), r.FormValue("intent_start"), r.FormValue("intent_end"), r.FormValue("intent_time"))
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", destination)
		return
	}
	http.Redirect(w, r, destination, http.StatusSeeOther)
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
	deviceID := s.anonymousDevice(w, r)
	if !s.limits.creationAllowed(r.RemoteAddr, deviceID) {
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
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/account-created?"+q.Encode())
		return
	}
	http.Redirect(w, r, "/account-created?"+q.Encode(), http.StatusSeeOther)
}

func (s *Server) signOut(w http.ResponseWriter, r *http.Request) {
	if m, csrf, err := s.currentMember(r); err == nil && m.ID > 0 && validCSRF(r, csrf) {
		if c, e := r.Cookie(sessionCookie); e == nil {
			s.app.SignOut(r.Context(), c.Value)
			log.Printf("security_event=member_session_revoked member_id=%d reason=sign_out", m.ID)
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
		Existing:    false,
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
		if r.Header.Get("HX-Request") == "true" {
			s.renderAuth(w, r, authPageData{IntentDate: r.FormValue("date"), IntentStart: strconv.Itoa(start), IntentEnd: strconv.Itoa(end), IntentTime: selectedMinuteQueryFromIntervals(intervals)})
			return
		}
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
	cookie, cookieErr := r.Cookie(sessionCookie)
	if cookieErr != nil || member.ID == 0 || s.app.SaveAttendanceIntervalsForToken(r.Context(), cookie.Value, r.FormValue("date"), intervals) != nil {
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
	_, csrf, err := s.currentMember(r)
	if err != nil {
		http.Error(w, "Sign in required", 401)
		return
	}
	if err := r.ParseForm(); err != nil || !validCSRF(r, csrf) {
		http.Error(w, "Invalid form", 403)
		return
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || s.app.RemoveAttendanceForToken(r.Context(), cookie.Value, r.FormValue("date")) != nil {
		http.Error(w, "Attendance could not be removed.", http.StatusConflict)
		return
	}
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
	w.Header().Add("Vary", "HX-Request")
	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Cache-Control", "private, no-store")
		s.render(w, "auth-surface", data)
		return
	}
	s.render(w, "auth", data)
}

func validCSRF(r *http.Request, want string) bool {
	got := r.FormValue("csrf")
	return want != "" && len(got) == len(want) && subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

const anonymousDeviceCookie = "squash_device"

func (s *Server) anonymousDevice(w http.ResponseWriter, r *http.Request) string {
	if cookie, err := r.Cookie(anonymousDeviceCookie); err == nil {
		parts := strings.Split(cookie.Value, ".")
		if len(parts) == 2 {
			mac := hmac.New(sha256.New, s.deviceKey)
			mac.Write([]byte(parts[0]))
			want := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
			if hmac.Equal([]byte(parts[1]), []byte(want)) {
				if raw, err := base64.RawURLEncoding.DecodeString(parts[0]); err == nil && len(raw) == 32 {
					return parts[0]
				}
			}
		}
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return ""
	}
	id := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, s.deviceKey)
	mac.Write([]byte(id))
	value := id + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	http.SetCookie(w, &http.Cookie{Name: anonymousDeviceCookie, Value: value, Path: "/", MaxAge: 365 * 24 * 60 * 60, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: s.app.SecureCookies()})
	return id
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
