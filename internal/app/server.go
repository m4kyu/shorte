package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"golang.org/x/crypto/bcrypt"
)

type Server struct {
	cfg   Config
	repo  *Repo
	cache *Cache
	mux   *http.ServeMux
	tpl   *template.Template
}

func NewServer(cfg Config) (*Server, error) {
	db, err := sql.Open("pgx", cfg.PostgresDSN)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	repo := NewRepo(db)
	cache := NewCache(cfg)
	tpl, err := template.New("pages").Parse(dashboardHTML + loginHTML + registerHTML)
	if err != nil {
		return nil, err
	}

	s := &Server{cfg: cfg, repo: repo, cache: cache, mux: http.NewServeMux(), tpl: tpl}
	s.routes()
	return s, nil
}

func (s *Server) Close() {
	_ = s.repo.Close()
	_ = s.cache.Close()
}

func (s *Server) Router() http.Handler { return s.mux }

func (s *Server) routes() {
	s.mux.HandleFunc("/health/live", s.handleLive)
	s.mux.HandleFunc("/health/ready", s.handleReady)
	s.mux.HandleFunc("/", s.handleHome)
	s.mux.HandleFunc("/login", s.handleLoginPage)
	s.mux.HandleFunc("/register", s.handleRegisterPage)
	s.mux.HandleFunc("/api/v1/auth/register", s.handleRegister)
	s.mux.HandleFunc("/api/v1/auth/login", s.handleLogin)
	s.mux.HandleFunc("/api/v1/auth/logout", s.handleLogout)
	s.mux.HandleFunc("/api/v1/auth/me", s.handleMe)
	s.mux.HandleFunc("/api/v1/links", s.auth(s.handleLinks))
	s.mux.HandleFunc("/api/v1/links/", s.auth(s.handleLinkByCode))
	s.mux.HandleFunc(s.cfg.RedirectPrefix, s.handleRedirect)
}

func (s *Server) handleLive(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.repo.db.PingContext(ctx); err != nil {
		http.Error(w, "db down", http.StatusServiceUnavailable)
		return
	}
	if err := s.cache.rdb.Ping(ctx).Err(); err != nil {
		http.Error(w, "redis down", http.StatusServiceUnavailable)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	uid := s.currentUserID(r)
	data := map[string]any{
		"Authenticated": uid != 0,
		"UserID":        uid,
		"Email":         "",
	}
	if uid != 0 {
		if u, err := s.repo.GetUserByID(r.Context(), uid); err == nil {
			data["Email"] = u.Email
		}
	}
	s.renderPage(w, "dashboard", data)
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/login" {
		http.NotFound(w, r)
		return
	}
	uid := s.currentUserID(r)
	data := map[string]any{
		"Authenticated": uid != 0,
		"UserID":        uid,
		"Email":         "",
	}
	if uid != 0 {
		if u, err := s.repo.GetUserByID(r.Context(), uid); err == nil {
			data["Email"] = u.Email
		}
	}
	s.renderPage(w, "login", data)
}

func (s *Server) handleRegisterPage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/register" {
		http.NotFound(w, r)
		return
	}
	uid := s.currentUserID(r)
	data := map[string]any{
		"Authenticated": uid != 0,
		"UserID":        uid,
		"Email":         "",
	}
	if uid != 0 {
		if u, err := s.repo.GetUserByID(r.Context(), uid); err == nil {
			data["Email"] = u.Email
		}
	}
	s.renderPage(w, "register", data)
}

func (s *Server) renderPage(w http.ResponseWriter, page string, data map[string]any) {
	if err := s.tpl.ExecuteTemplate(w, page, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "hash error", http.StatusInternalServerError)
		return
	}
	u, err := s.repo.CreateUser(r.Context(), strings.TrimSpace(in.Email), string(hash))
	if err != nil {
		http.Error(w, "create user failed", http.StatusBadRequest)
		return
	}
	setSession(w, s.cfg.SessionKey, u.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": u.ID, "email": u.Email})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var in struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	u, err := s.repo.GetUserByEmail(r.Context(), strings.TrimSpace(in.Email))
	if err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(in.Password)); err != nil {
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	setSession(w, s.cfg.SessionKey, u.ID)
	writeJSON(w, http.StatusOK, map[string]any{"id": u.ID, "email": u.Email})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	clearSession(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	uid := s.currentUserID(r)
	if uid == 0 {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	u, err := s.repo.GetUserByID(r.Context(), uid)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": u.ID, "email": u.Email})
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.currentUserID(r) == 0 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (s *Server) currentUserID(r *http.Request) int64 {
	v, ok := getSession(r, s.cfg.SessionKey)
	if !ok {
		return 0
	}
	id, _ := strconv.ParseInt(v, 10, 64)
	return id
}

func (s *Server) handleLinks(w http.ResponseWriter, r *http.Request) {
	uid := s.currentUserID(r)
	switch r.Method {
	case http.MethodPost:
		var in struct {
			LongURL   string `json:"long_url"`
			ExpiresAt string `json:"expires_at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := validateURL(in.LongURL); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var expiresAt *time.Time
		if in.ExpiresAt != "" {
			t, err := time.Parse(time.RFC3339, in.ExpiresAt)
			if err != nil {
				http.Error(w, "invalid expires_at", http.StatusBadRequest)
				return
			}
			expiresAt = &t
		}
		var l Link
		for range 5 {
			code, _ := generateCode(8)
			l, _ = s.repo.CreateLink(r.Context(), uid, code, in.LongURL, expiresAt)
			if l.ID != 0 {
				break
			}
		}
		if l.ID == 0 {
			http.Error(w, "failed to create link", http.StatusInternalServerError)
			return
		}
		_ = s.cache.SetLink(r.Context(), l.Code, CacheLink{LongURL: l.LongURL, IsActive: l.IsActive, ExpiresAt: l.ExpiresAt})
		writeJSON(w, http.StatusCreated, map[string]any{
			"link":       l,
			"short_url":  strings.TrimSuffix(s.cfg.BaseURL, "/") + s.cfg.RedirectPrefix + l.Code,
			"public_url": strings.TrimSuffix(s.cfg.BaseURL, "/") + "/r/" + l.Code,
		})
	case http.MethodGet:
		limit := 50
		offset := 0
		links, err := s.repo.ListLinksByOwner(r.Context(), uid, limit, offset)
		if err != nil {
			http.Error(w, "query failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": links})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleLinkByCode(w http.ResponseWriter, r *http.Request) {
	uid := s.currentUserID(r)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/links/")
	if path == "" {
		http.NotFound(w, r)
		return
	}
	parts := strings.Split(path, "/")
	code := parts[0]

	if len(parts) > 1 && parts[1] == "stats" && r.Method == http.MethodGet {
		from, err := parseDate(r.URL.Query().Get("from"))
		if err != nil {
			http.Error(w, "invalid from", http.StatusBadRequest)
			return
		}
		to, err := parseDate(r.URL.Query().Get("to"))
		if err != nil {
			http.Error(w, "invalid to", http.StatusBadRequest)
			return
		}
		stats, err := s.repo.GetStats(r.Context(), code, from, to)
		if err != nil {
			http.Error(w, "stats failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": stats})
		return
	}

	switch r.Method {
	case http.MethodGet:
		l, err := s.repo.GetLinkByCode(r.Context(), code)
		if err != nil || l.OwnerUserID != uid {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, l)
	case http.MethodPatch:
		var in struct {
			LongURL   string `json:"long_url"`
			IsActive  *bool  `json:"is_active"`
			ExpiresAt string `json:"expires_at"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if in.LongURL != "" {
			if err := validateURL(in.LongURL); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		cur, err := s.repo.GetLinkByCode(r.Context(), code)
		if err != nil || cur.OwnerUserID != uid {
			http.NotFound(w, r)
			return
		}
		if in.LongURL == "" {
			in.LongURL = cur.LongURL
		}
		isActive := cur.IsActive
		if in.IsActive != nil {
			isActive = *in.IsActive
		}
		expiresAt := cur.ExpiresAt
		if in.ExpiresAt != "" {
			t, err := time.Parse(time.RFC3339, in.ExpiresAt)
			if err != nil {
				http.Error(w, "invalid expires_at", http.StatusBadRequest)
				return
			}
			expiresAt = &t
		}
		l, err := s.repo.UpdateLink(r.Context(), uid, code, in.LongURL, isActive, expiresAt)
		if err != nil {
			http.Error(w, "update failed", http.StatusBadRequest)
			return
		}
		_ = s.cache.SetLink(r.Context(), l.Code, CacheLink{LongURL: l.LongURL, IsActive: l.IsActive, ExpiresAt: l.ExpiresAt})
		writeJSON(w, http.StatusOK, l)
	case http.MethodDelete:
		if err := s.repo.DisableLink(r.Context(), uid, code); err != nil {
			if errors.Is(err, ErrNotFound) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, "delete failed", http.StatusInternalServerError)
			return
		}
		_ = s.cache.DeleteLink(r.Context(), code)
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleRedirect(w http.ResponseWriter, r *http.Request) {
	if !(strings.HasPrefix(r.URL.Path, "/r/") || strings.HasPrefix(r.URL.Path, s.cfg.RedirectPrefix)) {
		http.NotFound(w, r)
		return
	}
	code := strings.TrimPrefix(r.URL.Path, "/r/")
	if code == r.URL.Path {
		code = strings.TrimPrefix(r.URL.Path, s.cfg.RedirectPrefix)
	}
	if code == "" {
		http.NotFound(w, r)
		return
	}
	cl, hit, err := s.cache.GetLink(r.Context(), code)
	if err == nil && hit {
		if !cl.IsActive || (cl.ExpiresAt != nil && time.Now().After(*cl.ExpiresAt)) {
			http.NotFound(w, r)
			return
		}
		_ = s.cache.EnqueueClick(r.Context(), code, time.Now())
		http.Redirect(w, r, cl.LongURL, http.StatusFound)
		return
	}

	l, err := s.repo.GetLinkByCode(r.Context(), code)
	if err != nil || !l.IsActive || (l.ExpiresAt != nil && time.Now().After(*l.ExpiresAt)) {
		http.NotFound(w, r)
		return
	}
	_ = s.cache.SetLink(r.Context(), code, CacheLink{LongURL: l.LongURL, IsActive: l.IsActive, ExpiresAt: l.ExpiresAt})
	_ = s.cache.EnqueueClick(r.Context(), code, time.Now())
	http.Redirect(w, r, l.LongURL, http.StatusFound)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

const dashboardHTML = `{{define "dashboard"}}<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width,initial-scale=1"/>
  <title>shorte - dashboard</title>
  <style>
    body { font-family: sans-serif; max-width: 900px; margin: 24px auto; padding: 0 16px; background: #f8fafc; color: #0f172a; }
    h1 { margin: 0 0 6px; }
    h3 { margin: 0 0 8px; font-size: 16px; }
    .grid { display: grid; grid-template-columns: 1fr; gap: 12px; }
    .centered { max-width: 520px; margin: 0 auto; width: 100%; }
    .card { background: #fff; border: 1px solid #e2e8f0; border-radius: 10px; padding: 12px; }
    .row { margin-bottom: 10px; }
    input, button { box-sizing: border-box; padding: 8px; margin: 4px 0; width: 100%; border: 1px solid #cbd5e1; border-radius: 8px; }
    button { cursor: pointer; background: #0f172a; color: #fff; border: 0; }
    button.secondary { background: #334155; }
    table { width: 100%; border-collapse: collapse; font-size: 14px; }
    th, td { text-align: left; border-bottom: 1px solid #e2e8f0; padding: 6px; vertical-align: top; }
    pre { background: #0f172a; color: #e2e8f0; padding: 10px; border-radius: 8px; overflow: auto; }
    .header { display: flex; align-items: center; justify-content: space-between; margin-bottom: 10px; }
    .badge { display: inline-block; padding: 4px 8px; border-radius: 999px; font-size: 12px; background: #e2e8f0; }
    .muted { color: #475569; }
    .actions a { margin-left: 8px; }
  </style>
</head>
<body>
  <div class="header">
    <h1>shorte</h1>
    <div>
      {{if .Authenticated}}
      <span class="badge">Logged in</span>
      <span class="muted" id="statusEmail">{{.Email}}</span>
      <button class="secondary" style="width:auto;" onclick="logout()">Logout</button>
      {{else}}
      <span class="badge">Logged out</span>
      <span class="actions"><a href="/login">Login</a><a href="/register">Register</a></span>
      {{end}}
    </div>
  </div>
  {{if .Authenticated}}
  <div class="grid">
    <div class="card centered">
      <div class="row">
        <h3>Create Short Link</h3>
        <input id="url" placeholder="https://example.com"/>
        <input id="expires" placeholder="expires_at RFC3339 (optional)"/>
        <button onclick="createLink()">Create</button>
      </div>
    </div>
  </div>
  <div class="card" style="margin-top:12px;">
    <h3>Your Links</h3>
    <button onclick="loadLinks()">Refresh Links</button>
    <table id="linksTbl">
      <thead>
        <tr><th>Code</th><th>Long URL</th><th>Active</th><th>Short URL</th><th>Stats</th></tr>
      </thead>
      <tbody></tbody>
    </table>
  </div>
  {{else}}
  <div class="card">
    <h3>Sign in required</h3>
    <p>Login or register to create and manage your links.</p>
    <a href="/login">Go to login</a> | <a href="/register">Go to register</a>
  </div>
  {{end}}
  <div class="card" style="margin-top:12px;">
    <h3>API Response</h3>
    <pre id="out"></pre>
  </div>
  <script>
    const out = document.getElementById('out');
    const val = (id) => document.getElementById(id).value;
    async function call(path, method, body, quiet=false) {
      const r = await fetch(path, {method, headers: {'content-type':'application/json'}, body: body ? JSON.stringify(body) : undefined});
      const t = await r.text();
      if (!quiet && out) out.textContent = t;
      let json = null;
      try { json = JSON.parse(t); } catch (_e) {}
      return {status: r.status, json, text: t};
    }
    async function logout(){ await call('/api/v1/auth/logout','POST'); location.reload(); }
    async function createLink(){
      const body = {long_url:val('url')};
      if (val('expires')) body.expires_at = val('expires');
      await call('/api/v1/links','POST',body);
      await loadLinks();
    }
    async function loadLinks() {
      const res = await call('/api/v1/links','GET',null,true);
      const tb = document.querySelector('#linksTbl tbody');
      if (!tb) return;
      tb.innerHTML = '';
      if (!res.json || !res.json.items) return;
      for (const l of res.json.items) {
        const tr = document.createElement('tr');
        const shortUrl = location.origin + '/r/' + l.code;
        tr.innerHTML =
          '<td>'+l.code+'</td>' +
          '<td><a href="'+l.long_url+'" target="_blank">'+l.long_url+'</a></td>' +
          '<td>'+String(l.is_active)+'</td>' +
          '<td><a href="'+shortUrl+'" target="_blank">'+shortUrl+'</a></td>' +
          '<td><button class="secondary" onclick="loadStats(\''+l.code+'\')">Last 7d</button></td>';
        tb.appendChild(tr);
      }
    }
    async function loadStats(code) {
      const to = new Date();
      const from = new Date(Date.now()-6*24*3600*1000);
      const fmt = (d) => d.toISOString().slice(0,10);
      await call('/api/v1/links/'+code+'/stats?from='+fmt(from)+'&to='+fmt(to),'GET');
    }
    async function loadProfile() {
      const res = await call('/api/v1/auth/me','GET',null,true);
      if (res.status !== 200 || !res.json) return;
      const statusEmail = document.getElementById('statusEmail');
      if (statusEmail) statusEmail.textContent = res.json.email;
    }
    loadProfile();
    loadLinks();
  </script>
</body>
</html>{{end}}`

const loginHTML = `{{define "login"}}<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width,initial-scale=1"/>
  <title>shorte - login</title>
  <style>
    body { font-family: sans-serif; max-width: 520px; margin: 24px auto; padding: 0 16px; background: #f8fafc; color: #0f172a; }
    .card { background: #fff; border: 1px solid #e2e8f0; border-radius: 10px; padding: 12px; }
    input, button { padding: 8px; margin: 4px 0; width: 100%; border: 1px solid #cbd5e1; border-radius: 8px; }
    button { cursor: pointer; background: #0f172a; color: #fff; border: 0; }
    pre { background: #0f172a; color: #e2e8f0; padding: 10px; border-radius: 8px; overflow: auto; }
  </style>
</head>
<body>
  <h1>Login</h1>
  <div class="card">
    <input id="email" placeholder="email@example.com"/>
    <input id="password" type="password" placeholder="password"/>
    <button onclick="login()">Login</button>
    <p><a href="/register">Create account</a> | <a href="/">Dashboard</a></p>
  </div>
  <div class="card" style="margin-top:12px;">
    <h3>API Response</h3>
    <pre id="out"></pre>
  </div>
  <script>
    const out = document.getElementById('out');
    const val = (id) => document.getElementById(id).value;
    async function call(path, method, body) {
      const r = await fetch(path, {method, headers: {'content-type':'application/json'}, body: body ? JSON.stringify(body) : undefined});
      const t = await r.text();
      out.textContent = t;
      let json = null;
      try { json = JSON.parse(t); } catch (_e) {}
      return {status: r.status, json, text: t};
    }
    async function login(){
      const res = await call('/api/v1/auth/login','POST',{email:val('email'),password:val('password')});
      if (res.status === 200) location.href = '/';
    }
  </script>
</body>
</html>{{end}}`

const registerHTML = `{{define "register"}}<!doctype html>
<html>
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width,initial-scale=1"/>
  <title>shorte - register</title>
  <style>
    body { font-family: sans-serif; max-width: 520px; margin: 24px auto; padding: 0 16px; background: #f8fafc; color: #0f172a; }
    .card { background: #fff; border: 1px solid #e2e8f0; border-radius: 10px; padding: 12px; }
    input, button { padding: 8px; margin: 4px 0; width: 100%; border: 1px solid #cbd5e1; border-radius: 8px; }
    button { cursor: pointer; background: #0f172a; color: #fff; border: 0; }
    pre { background: #0f172a; color: #e2e8f0; padding: 10px; border-radius: 8px; overflow: auto; }
  </style>
</head>
<body>
  <h1>Register</h1>
  <div class="card">
    <input id="email" placeholder="email@example.com"/>
    <input id="password" type="password" placeholder="password"/>
    <button onclick="register()">Register</button>
    <p><a href="/login">Already have an account</a> | <a href="/">Dashboard</a></p>
  </div>
  <div class="card" style="margin-top:12px;">
    <h3>API Response</h3>
    <pre id="out"></pre>
  </div>
  <script>
    const out = document.getElementById('out');
    const val = (id) => document.getElementById(id).value;
    async function call(path, method, body) {
      const r = await fetch(path, {method, headers: {'content-type':'application/json'}, body: body ? JSON.stringify(body) : undefined});
      const t = await r.text();
      out.textContent = t;
      let json = null;
      try { json = JSON.parse(t); } catch (_e) {}
      return {status: r.status, json, text: t};
    }
    async function register(){
      const res = await call('/api/v1/auth/register','POST',{email:val('email'),password:val('password')});
      if (res.status === 201) location.href = '/';
    }
  </script>
</body>
</html>{{end}}`
