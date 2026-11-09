package webserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"time"
)

type Download struct {
	Name string
	MIME string
	Data []byte
}
type Options struct {
	Token   string
	Origin  string
	Secure  bool
	Assets  fs.FS
	Methods map[string]any
	Export  func(context.Context, string, string, string, string) (Download, error)
}
type session struct{ expires time.Time }
type attempt struct {
	count int
	since time.Time
}
type Server struct {
	options   Options
	token     [32]byte
	mu        sync.Mutex
	sessions  map[string]session
	attempts  map[string]attempt
	listeners map[chan []byte]bool
	work      chan struct{}
}

func New(options Options) (*Server, error) {
	if len(options.Token) < 32 {
		return nil, errors.New("web login token must contain at least 32 characters")
	}
	if options.Origin != "" {
		origin, err := url.Parse(options.Origin)
		if err != nil || origin.Host == "" || origin.User != nil || origin.Path != "" || origin.RawQuery != "" || origin.Fragment != "" || origin.Scheme != "http" && origin.Scheme != "https" {
			return nil, errors.New("web origin must be an HTTP(S) origin without a path")
		}
	}
	return &Server{options: options, token: sha256.Sum256([]byte(options.Token)), sessions: map[string]session{}, attempts: map[string]attempt{}, listeners: map[chan []byte]bool{}, work: make(chan struct{}, 4)}, nil
}
func (s *Server) Publish(value any) {
	data, err := json.Marshal(value)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for listener := range s.listeners {
		select {
		case listener <- data:
		default:
			select {
			case <-listener:
			default:
			}
			select {
			case listener <- data:
			default:
			}
		}
	}
}
func (s *Server) authenticated(r *http.Request) bool {
	cookie, err := r.Cookie("kiko_session")
	if err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.sessions[cookie.Value]
	if !ok || time.Now().After(entry.expires) {
		delete(s.sessions, cookie.Value)
		return false
	}
	return true
}
func (s *Server) origin(r *http.Request) bool {
	expected := s.options.Origin
	if expected == "" {
		scheme := "http"
		if s.options.Secure || r.TLS != nil {
			scheme = "https"
		}
		expected = scheme + "://" + r.Host
	}
	return r.Header.Get("Origin") == expected
}
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; worker-src 'self' blob:; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'")
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Cache-Control", "no-store")
	}
	if r.URL.Path == "/web-runtime.js" {
		w.Header().Set("Content-Type", "application/javascript")
		io.WriteString(w, "window.kikoWeb=true;")
		return
	}
	if r.URL.Path == "/api/login" {
		s.login(w, r)
		return
	}
	if r.URL.Path == "/api/session" {
		json.NewEncoder(w).Encode(map[string]bool{"authenticated": s.authenticated(r)})
		return
	}
	if strings.HasPrefix(r.URL.Path, "/api/") {
		if !s.authenticated(r) {
			http.Error(w, "Sign in to continue.", http.StatusUnauthorized)
			return
		}
		if r.Method != "GET" && !s.origin(r) {
			http.Error(w, "Invalid request origin.", http.StatusForbidden)
			return
		}
		switch r.URL.Path {
		case "/api/rpc":
			s.rpc(w, r)
		case "/api/events":
			s.events(w, r)
		case "/api/export":
			s.export(w, r)
		case "/api/logout":
			if r.Method != "POST" {
				http.Error(w, "POST required", 405)
				return
			}
			cookie, _ := r.Cookie("kiko_session")
			s.mu.Lock()
			delete(s.sessions, cookie.Value)
			s.mu.Unlock()
			http.SetCookie(w, &http.Cookie{Name: "kiko_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.options.Secure})
			w.WriteHeader(204)
		default:
			http.NotFound(w, r)
		}
		return
	}
	if s.options.Assets == nil {
		http.NotFound(w, r)
		return
	}
	if r.URL.Path == "/" {
		raw, err := fs.ReadFile(s.options.Assets, "index.html")
		if err != nil {
			http.Error(w, "Build the frontend before starting Kiko web.", 503)
			return
		}
		html := strings.Replace(string(raw), "<head>", "<head><script src=\"/web-runtime.js\"></script>", 1)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		io.WriteString(w, html)
		return
	}
	http.FileServer(http.FS(s.options.Assets)).ServeHTTP(w, r)
}
func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	if !s.origin(r) {
		http.Error(w, "Invalid request origin.", 403)
		return
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	s.mu.Lock()
	for key, value := range s.attempts {
		if time.Since(value.since) > time.Minute {
			delete(s.attempts, key)
		}
	}
	if _, exists := s.attempts[ip]; !exists && len(s.attempts) >= 4096 {
		s.mu.Unlock()
		http.Error(w, "Login capacity reached. Retry shortly.", 429)
		return
	}
	entry := s.attempts[ip]
	if time.Since(entry.since) > time.Minute {
		entry = attempt{since: time.Now()}
	}
	entry.count++
	s.attempts[ip] = entry
	blocked := entry.count > 10
	s.mu.Unlock()
	if blocked {
		w.Header().Set("Retry-After", "60")
		http.Error(w, "Too many login attempts. Retry in a minute.", 429)
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input) != nil {
		http.Error(w, "Invalid login", 400)
		return
	}
	candidate := sha256.Sum256([]byte(input.Token))
	if subtle.ConstantTimeCompare(candidate[:], s.token[:]) != 1 {
		http.Error(w, "Invalid login token.", 401)
		return
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		http.Error(w, "Couldn’t create a session", 500)
		return
	}
	id := base64.RawURLEncoding.EncodeToString(bytes)
	s.mu.Lock()
	for key, value := range s.sessions {
		if time.Now().After(value.expires) {
			delete(s.sessions, key)
		}
	}
	if len(s.sessions) >= 64 {
		s.mu.Unlock()
		http.Error(w, "Session limit reached. Sign out an existing session.", 429)
		return
	}
	s.sessions[id] = session{expires: time.Now().Add(12 * time.Hour)}
	delete(s.attempts, ip)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "kiko_session", Value: id, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode, Secure: s.options.Secure, MaxAge: 12 * 60 * 60})
	w.WriteHeader(204)
}
func (s *Server) rpc(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	select {
	case s.work <- struct{}{}:
		defer func() { <-s.work }()
	case <-r.Context().Done():
		return
	}
	defer func() {
		if recover() != nil {
			http.Error(w, "Couldn’t complete this action.", 500)
		}
	}()
	var input struct {
		Method string            `json:"method"`
		Args   []json.RawMessage `json:"args"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 30*1024*1024))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(new(any)) != io.EOF {
		http.Error(w, "Invalid request", 400)
		return
	}
	method, ok := s.options.Methods[input.Method]
	if !ok {
		http.Error(w, "Unknown action", 404)
		return
	}
	function := reflect.ValueOf(method)
	signature := function.Type()
	if signature.Kind() != reflect.Func || signature.NumIn() != len(input.Args) {
		http.Error(w, "Invalid arguments", 400)
		return
	}
	arguments := make([]reflect.Value, len(input.Args))
	for i, raw := range input.Args {
		value := reflect.New(signature.In(i))
		if err := json.Unmarshal(raw, value.Interface()); err != nil {
			http.Error(w, "Invalid arguments", 400)
			return
		}
		arguments[i] = value.Elem()
	}
	values := function.Call(arguments)
	var result any
	var problem string
	for _, value := range values {
		if value.Type().Implements(reflect.TypeFor[error]()) {
			if !value.IsNil() {
				problem = value.Interface().(error).Error()
			}
		} else {
			result = value.Interface()
		}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"result": result, "error": problem})
}
func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "GET required", 405)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unavailable", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	listener := make(chan []byte, 2)
	s.mu.Lock()
	if len(s.listeners) >= 16 {
		s.mu.Unlock()
		http.Error(w, "Too many event connections", 429)
		return
	}
	s.listeners[listener] = true
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.listeners, listener); s.mu.Unlock() }()
	controller := http.NewResponseController(w)
	_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
	io.WriteString(w, ": connected\n\n")
	flusher.Flush()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case message := <-listener:
			if !s.authenticated(r) {
				return
			}
			_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := w.Write(append(append([]byte("event: kiko:message\ndata: "), message...), []byte("\n\n")...)); err != nil {
				return
			}
			flusher.Flush()
		case <-ticker.C:
			if !s.authenticated(r) {
				return
			}
			_ = controller.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if _, err := io.WriteString(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
func (s *Server) export(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" || s.options.Export == nil {
		http.Error(w, "Export unavailable", 405)
		return
	}
	download, err := s.options.Export(r.Context(), r.URL.Query().Get("kind"), r.URL.Query().Get("id"), r.URL.Query().Get("format"), r.URL.Query().Get("tab"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	w.Header().Set("Content-Type", download.MIME)
	w.Header().Set("Content-Disposition", `attachment; filename*=UTF-8''`+url.PathEscape(download.Name))
	w.Write(download.Data)
}
