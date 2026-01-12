package srv

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"runtime"

	"srv.exe.dev/db"
)

type Server struct {
	DB           *sql.DB
	Hostname     string
	TemplatesDir string
	StaticDir    string
}

func New(dbPath, hostname string) (*Server, error) {
	_, thisFile, _, _ := runtime.Caller(0)
	baseDir := filepath.Dir(thisFile)
	srv := &Server{
		Hostname:     hostname,
		TemplatesDir: filepath.Join(baseDir, "templates"),
		StaticDir:    filepath.Join(baseDir, "static"),
	}
	if err := srv.setUpDatabase(dbPath); err != nil {
		return nil, err
	}
	return srv, nil
}

func (s *Server) setUpDatabase(dbPath string) error {
	wdb, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("failed to open db: %w", err)
	}
	s.DB = wdb
	if err := db.RunMigrations(wdb); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}
	return nil
}

func (s *Server) Serve(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.HandleIndex)
	mux.HandleFunc("GET /extension", s.HandleExtensionPage)
	mux.HandleFunc("GET /api/bookmarks", s.HandleListBookmarks)
	mux.HandleFunc("POST /api/bookmarks", s.cors(s.HandleCreateBookmark))
	mux.HandleFunc("GET /api/bookmarks/{id}", s.HandleGetBookmark)
	mux.HandleFunc("PUT /api/bookmarks/{id}", s.HandleUpdateBookmark)
	mux.HandleFunc("DELETE /api/bookmarks/{id}", s.HandleDeleteBookmark)
	mux.HandleFunc("GET /api/tags", s.HandleListTags)
	mux.HandleFunc("POST /api/tags", s.HandleCreateTag)
	mux.HandleFunc("GET /api/collections", s.HandleListCollections)
	mux.HandleFunc("POST /api/collections", s.HandleCreateCollection)
	mux.HandleFunc("GET /api/search", s.HandleSearch)
	mux.HandleFunc("GET /api/web-search", s.HandleWebSearch)
	mux.HandleFunc("POST /api/fetch-metadata", s.HandleFetchMetadata)
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(s.StaticDir))))
	mux.HandleFunc("OPTIONS /api/bookmarks", s.cors(func(w http.ResponseWriter, r *http.Request) {}))
	slog.Info("starting server", "addr", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) HandleIndex(w http.ResponseWriter, r *http.Request) {
	if err := s.renderTemplate(w, "index.html", nil); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal error", 500)
	}
}

func (s *Server) renderTemplate(w http.ResponseWriter, name string, data any) error {
	path := filepath.Join(s.TemplatesDir, name)
	tmpl, err := template.ParseFiles(path)
	if err != nil {
		return fmt.Errorf("parse template %q: %w", name, err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return tmpl.Execute(w, data)
}

func writeJSON(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func (s *Server) cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == "OPTIONS" {
			w.WriteHeader(200)
			return
		}
		next(w, r)
	}
}

func (s *Server) HandleExtensionPage(w http.ResponseWriter, r *http.Request) {
	if err := s.renderTemplate(w, "extension.html", nil); err != nil {
		slog.Warn("render template", "error", err)
		http.Error(w, "Internal error", 500)
	}
}
