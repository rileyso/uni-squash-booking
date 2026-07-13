// Package web provides the progressive HTML adapter.
package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"time"

	"github.com/rileyso/uni-squash-booking/internal/app"
)

//go:embed templates/*.html static/*
var assets embed.FS

type Server struct {
	app      *app.Service
	template *template.Template
}

func New(application *app.Service) (*Server, error) {
	tmpl, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{app: application, template: tmpl}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /", s.index)
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
	data := struct {
		Synthetic bool
		Dashboard app.Dashboard
	}{s.app.Synthetic(), dashboard}
	if err := s.template.ExecuteTemplate(w, "index", data); err != nil {
		http.Error(w, "The page is temporarily unavailable.", http.StatusInternalServerError)
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
