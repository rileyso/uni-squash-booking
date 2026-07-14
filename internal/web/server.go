// Package web provides the progressive HTML adapter.
package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
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

func New(application *app.Service) (*Server, error) {
	tmpl, err := template.ParseFS(assets, "templates/*.html")
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
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	dashboard, err := s.app.Dashboard(r.Context(), r.URL.Query().Get("date"), r.URL.Query().Get("time"))
	if err != nil {
		http.Error(w, "Current attendance could not be loaded. Please try again.", http.StatusServiceUnavailable)
		return
	}
	member, csrf, _ := s.currentMember(r)
	plans, _ := s.app.Plans(r.Context(), member)
	data := struct {
		Synthetic bool
		Dashboard app.Dashboard
		Member    app.Member
		CSRF      string
		Plans     []app.Plan
	}{s.app.Synthetic(), dashboard, member, csrf, plans}
	if err := s.template.ExecuteTemplate(w, "index", data); err != nil {
		http.Error(w, "The page is temporarily unavailable.", http.StatusInternalServerError)
	}
}

const sessionCookie = "squash_member"

type authPageData struct {
	Synthetic                      bool
	Create                         bool
	Error, IntentDate, IntentStart string
}

func (s *Server) signInPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "auth", authPageData{Synthetic: s.app.Synthetic(), IntentDate: r.URL.Query().Get("intent_date"), IntentStart: r.URL.Query().Get("intent_start")})
}
func (s *Server) createAccountPage(w http.ResponseWriter, r *http.Request) {
	s.render(w, "auth", authPageData{Synthetic: s.app.Synthetic(), Create: true, IntentDate: r.URL.Query().Get("intent_date"), IntentStart: r.URL.Query().Get("intent_start")})
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
		s.render(w, "auth", authPageData{Synthetic: s.app.Synthetic(), Error: "Too many attempts. Try again later.", IntentDate: r.FormValue("intent_date"), IntentStart: r.FormValue("intent_start")})
		return
	}
	session, err := s.app.SignIn(r.Context(), username, r.FormValue("pin"))
	if err != nil {
		s.limits.loginFailed(username, r.RemoteAddr)
		w.WriteHeader(http.StatusUnauthorized)
		s.render(w, "auth", authPageData{Synthetic: s.app.Synthetic(), Error: "The username or PIN was not recognised.", IntentDate: r.FormValue("intent_date"), IntentStart: r.FormValue("intent_start")})
		return
	}
	s.limits.loginSucceeded(username)
	s.setSession(w, session)
	http.Redirect(w, r, intentDestination(r.FormValue("intent_date"), r.FormValue("intent_start")), http.StatusSeeOther)
}

func (s *Server) accountCreated(w http.ResponseWriter, r *http.Request) {
	member, _, err := s.currentMember(r)
	if err != nil {
		http.Redirect(w, r, "/sign-in", http.StatusSeeOther)
		return
	}
	s.render(w, "account-created", struct {
		Synthetic bool
		Member    app.Member
		Continue  string
	}{s.app.Synthetic(), member, intentDestination(r.URL.Query().Get("intent_date"), r.URL.Query().Get("intent_start"))})
}

func (s *Server) createAccount(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", 400)
		return
	}
	if !s.limits.creationAllowed(r.RemoteAddr, r.UserAgent()) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(http.StatusTooManyRequests)
		s.render(w, "auth", authPageData{Synthetic: s.app.Synthetic(), Create: true, Error: "Too many account attempts. Try again later.", IntentDate: r.FormValue("intent_date"), IntentStart: r.FormValue("intent_start")})
		return
	}
	pin := r.FormValue("pin")
	if pin != r.FormValue("pin_confirm") {
		w.WriteHeader(422)
		s.render(w, "auth", authPageData{Synthetic: s.app.Synthetic(), Create: true, Error: "The PINs do not match.", IntentDate: r.FormValue("intent_date"), IntentStart: r.FormValue("intent_start")})
		return
	}
	session, err := s.app.CreateAccount(r.Context(), r.FormValue("display_name"), pin, r.FormValue("member_status"), r.FormValue("privacy_ack") != "")
	if err != nil {
		w.WriteHeader(422)
		s.render(w, "auth", authPageData{Synthetic: s.app.Synthetic(), Create: true, Error: "Check the account details and privacy acknowledgement.", IntentDate: r.FormValue("intent_date"), IntentStart: r.FormValue("intent_start")})
		return
	}
	s.setSession(w, session)
	q := url.Values{"intent_date": {r.FormValue("intent_date")}, "intent_start": {r.FormValue("intent_start")}}
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
type attendanceData struct {
	Synthetic         bool
	Member            app.Member
	CSRF, Error, Date string
	Start, End        int
	Existing          bool
	Review            bool
	StartLabel        string
	EndLabel          string
	Duration          string
	Times             []timeOption
	DateHeading       string
	AllowedStart      int
	AllowedEnd        int
	Availability      []bool
}

func (s *Server) attendancePage(w http.ResponseWriter, r *http.Request) {
	member, csrf, err := s.currentMember(r)
	if err != nil {
		q := url.Values{"intent_date": {r.URL.Query().Get("date")}, "intent_start": {r.URL.Query().Get("start")}}
		http.Redirect(w, r, "/sign-in?"+q.Encode(), http.StatusSeeOther)
		return
	}
	date := r.URL.Query().Get("date")
	start, _ := strconv.Atoi(r.URL.Query().Get("start"))
	if start < 600 || start > 1260 || start%30 != 0 {
		start = 1020
	}
	end := start + 60
	if requestedEnd, err := strconv.Atoi(r.URL.Query().Get("end")); err == nil && requestedEnd > start && requestedEnd%30 == 0 {
		end = requestedEnd
	}
	existing, _ := s.app.PlanForDate(r.Context(), member, date)
	has := existing != nil
	if has {
		start, end = existing.StartMinute, existing.EndMinute
	}
	data := attendanceData{Synthetic: s.app.Synthetic(), Member: member, CSRF: csrf, Date: date, Start: start, End: end, Existing: has, Times: timeOptions()}
	s.decorateAttendance(r, &data)
	s.render(w, "attendance", data)
}

func (s *Server) reviewAttendance(w http.ResponseWriter, r *http.Request) {
	member, csrf, err := s.currentMember(r)
	if err != nil {
		http.Error(w, "Sign in required", http.StatusUnauthorized)
		return
	}
	if err := r.ParseForm(); err != nil || !validCSRF(r, csrf) {
		http.Error(w, "Invalid form", http.StatusForbidden)
		return
	}
	start, _ := strconv.Atoi(r.FormValue("start"))
	end, _ := strconv.Atoi(r.FormValue("end"))
	data := attendanceData{Synthetic: s.app.Synthetic(), Member: member, CSRF: csrf, Date: r.FormValue("date"), Start: start, End: end, Times: timeOptions()}
	s.decorateAttendance(r, &data)
	if err := s.app.ValidateAttendance(r.Context(), data.Date, start, end); err != nil {
		data.Error = "That interval is not continuously open for play. Choose another range."
		w.WriteHeader(http.StatusUnprocessableEntity)
		s.render(w, "attendance", data)
		return
	}
	data.Review = true
	data.StartLabel = fmtTime(start/60, start%60)
	data.EndLabel = fmtTime(end/60, end%60)
	data.Duration = formatDuration(end - start)
	s.render(w, "attendance", data)
}

func (s *Server) saveAttendance(w http.ResponseWriter, r *http.Request) {
	member, csrf, err := s.currentMember(r)
	if err != nil {
		http.Error(w, "Sign in required", 401)
		return
	}
	if err := r.ParseForm(); err != nil || !validCSRF(r, csrf) {
		http.Error(w, "Invalid form", 403)
		return
	}
	start, _ := strconv.Atoi(r.FormValue("start"))
	end, _ := strconv.Atoi(r.FormValue("end"))
	if err := s.app.SaveAttendance(r.Context(), member, r.FormValue("date"), start, end); err != nil {
		w.WriteHeader(409)
		data := attendanceData{Synthetic: s.app.Synthetic(), Member: member, CSRF: csrf, Error: "That interval is no longer continuously open for play. Choose another range.", Date: r.FormValue("date"), Start: start, End: end, Times: timeOptions()}
		s.decorateAttendance(r, &data)
		s.render(w, "attendance", data)
		return
	}
	http.Redirect(w, r, "/?date="+url.QueryEscape(r.FormValue("date")), http.StatusSeeOther)
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
func validCSRF(r *http.Request, want string) bool {
	return want != "" && strings.EqualFold(r.FormValue("csrf"), want)
}
func intentDestination(date, start string) string {
	if date == "" {
		return "/"
	}
	q := url.Values{"date": {date}, "start": {start}}
	return "/attendance/new?" + q.Encode()
}
func timeOptions() []timeOption {
	var result []timeOption
	for minute := 600; minute <= 1320; minute += 30 {
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

func (s *Server) decorateAttendance(r *http.Request, data *attendanceData) {
	if date, err := time.Parse("2006-01-02", data.Date); err == nil {
		data.DateHeading = date.Format("Monday 2 January")
	}
	data.AllowedStart, data.AllowedEnd = 600, 1320
	data.Availability = make([]bool, 24)
	segments, err := s.app.PlayableSegments(r.Context(), data.Date)
	if err != nil {
		return
	}
	for index := range data.Availability {
		minute := 600 + index*30
		for _, segment := range segments {
			if minute >= segment.StartMinute && minute < segment.EndMinute {
				data.Availability[index] = true
				break
			}
		}
	}
	for _, segment := range segments {
		if data.Start >= segment.StartMinute && data.Start < segment.EndMinute && segment.EndMinute-segment.StartMinute >= 60 {
			data.AllowedStart, data.AllowedEnd = segment.StartMinute, segment.EndMinute
			if data.End > data.AllowedEnd {
				data.End = data.AllowedEnd
			}
			break
		}
	}
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
