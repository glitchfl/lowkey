package fakeriot

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// rejected handshakes are expected in tests that check certificate trust
var quiet = log.New(io.Discard, "", 0)

type Options struct {
	// nil uses real time
	Clock interface{ Now() time.Time }
	// where the lockfile goes - empty makes a temporary folder
	Dir   string
	World *World
	// serve a SHA-1-signed certificate from an untrusted root instead of the usual one
	SHA1Cert bool
}

type Server struct {
	now     func() time.Time
	sha1    bool
	dir     string
	ownsDir bool
	certs   certs

	mu          sync.Mutex
	world       *World
	password    string
	lockPass    string
	access      string
	entitlement string
	tokenExp    time.Time
	entitleExp  time.Time
	log         []*Request
	faults      []*Fault
	inflight    int
	maxInflight int
	held        []*heldRequest
	ws          map[*wsConn]bool
	messages    int

	local     *http.Server
	localPort int
	remote    *http.Server
	remoteURL string
}

type Request struct {
	Seq      int
	Service  string // local / pd / glz / shared / static
	Region   string
	Shard    string
	Method   string
	Path     string
	Query    url.Values
	Header   http.Header
	Status   int
	Canceled bool
	Start    time.Time
	End      time.Time
	done     bool
}

type Fault struct {
	Service string // empty matches every service
	Path    string // prefix - empty matches every path
	Exact   bool   // match Path exactly instead of as a prefix
	Status  int
	Body    string
	Header  map[string]string
	Times   int // 0 keeps the fault until RemoveFaults
	Delay   time.Duration
	Hold    bool // wait for Release or ReleaseHeld or the client giving up
	Drop    bool // close the connection without answering
}

type heldRequest struct {
	req     *Request
	release chan struct{}
}

type URLs struct {
	PD     string
	GLZ    string
	Shared string
	Static string
}

func Start(opts Options) (*Server, error) {
	c, err := loadCerts()
	if err != nil {
		return nil, err
	}
	s := &Server{now: time.Now, sha1: opts.SHA1Cert, certs: c, ws: map[*wsConn]bool{}}
	if opts.Clock != nil {
		s.now = opts.Clock.Now
	}
	s.world = opts.World
	if s.world == nil {
		s.world = NewWorld(s.now())
	}
	s.dir = opts.Dir
	if s.dir == "" {
		s.dir, err = os.MkdirTemp("", "fakeriot")
		if err != nil {
			return nil, err
		}
		s.ownsDir = true
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	s.remoteURL = "http://" + ln.Addr().String()
	s.remote = &http.Server{Handler: http.HandlerFunc(s.serveRemote), ErrorLog: quiet}
	go s.remote.Serve(ln)

	if s.world.RiotClient {
		if err := s.startClient(); err != nil {
			s.Close()
			return nil, err
		}
	}
	return s, nil
}

func (s *Server) Close() {
	s.mu.Lock()
	local, remote := s.local, s.remote
	s.local = nil
	s.mu.Unlock()
	s.closeWS()
	if local != nil {
		local.Close()
	}
	if remote != nil {
		remote.Close()
	}
	s.ReleaseHeld()
	if s.ownsDir {
		os.RemoveAll(s.dir)
	}
}

func (s *Server) Lockfile() string { return filepath.Join(s.dir, "lockfile") }

func (s *Server) Roots() *x509.CertPool { return s.certs.pool }

// CAPEM is the root a separate lowkey process has to trust to reach the local API
func (s *Server) CAPEM() []byte { return s.certs.caPEM }

func (s *Server) URLs() URLs {
	return URLs{
		PD:     s.remoteURL + "/pd/{shard}",
		GLZ:    s.remoteURL + "/glz/{region}/{shard}",
		Shared: s.remoteURL + "/shared/{shard}",
		Static: s.remoteURL + "/static",
	}
}

func (s *Server) Port() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.localPort
}

// Update changes the world under the server's lock
func (s *Server) Update(f func(w *World)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	f(s.world)
}

func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, 0, len(s.log))
	for _, r := range s.log {
		out = append(out, *r)
	}
	return out
}

// Hidden lists the players in streamer mode - your own PUUID never counts as hidden
func (s *Server) Hidden() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for puuid, p := range s.world.Players {
		if p.Incognito && puuid != s.world.Self {
			out = append(out, puuid)
		}
	}
	return out
}

func (s *Server) Self() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.world.Self
}

// RemoteRequests leaves out the local API - it never leaves the PC
func (s *Server) RemoteRequests() []Request {
	var out []Request
	for _, r := range s.Requests() {
		if r.Service != "local" {
			out = append(out, r)
		}
	}
	return out
}

func (s *Server) MaxInFlight() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxInflight
}

func (s *Server) InFlight() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inflight
}

func (s *Server) AddFault(f Fault) {
	s.mu.Lock()
	defer s.mu.Unlock()
	fc := f
	s.faults = append(s.faults, &fc)
}

// Answered counts remote requests to a path by status
func (s *Server) Answered(path string, status int) int {
	n := 0
	for _, r := range s.RemoteRequests() {
		if r.Path == path && r.Status == status {
			n++
		}
	}
	return n
}

func (s *Server) RemoveFaults() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.faults = nil
}

// Held lists the requests waiting on a Hold fault - oldest first
func (s *Server) Held() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Request, len(s.held))
	for i, h := range s.held {
		out[i] = *h.req
	}
	return out
}

// Release lets the oldest held request for path answer
func (s *Server) Release(path string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, h := range s.held {
		if h.req.Path == path {
			close(h.release)
			s.held = append(s.held[:i:i], s.held[i+1:]...)
			return true
		}
	}
	return false
}

// ReleaseHeld lets every held response go and stops holding new ones until the next Hold fault
func (s *Server) ReleaseHeld() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, h := range s.held {
		close(h.release)
	}
	s.held = nil
	kept := s.faults[:0]
	for _, f := range s.faults {
		if !f.Hold {
			kept = append(kept, f)
		}
	}
	s.faults = kept
}

// SetLockfilePassword writes a lockfile whose password the local API won't accept
func (s *Server) SetLockfilePassword(pw string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lockPass = pw
	return s.writeLockfile()
}

// RotateTokens makes the tokens lowkey holds stale - the next entitlements call hands out new ones
func (s *Server) RotateTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.issueTokens()
}

// StopClient is the Riot Client quitting: the lockfile goes and the local API stops
func (s *Server) StopClient() {
	s.mu.Lock()
	local := s.local
	s.local = nil
	s.world.RiotClient = false
	os.Remove(s.Lockfile())
	s.mu.Unlock()
	s.closeWS()
	if local != nil {
		local.Close()
	}
}

// StartClient brings the Riot Client up on a fresh port with a fresh password
func (s *Server) StartClient() error {
	s.mu.Lock()
	s.world.RiotClient = true
	s.mu.Unlock()
	return s.startClient()
}

func (s *Server) Restart() error {
	s.StopClient()
	return s.StartClient()
}

func (s *Server) startClient() error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	cert := s.certs.server
	if s.sha1 {
		cert = s.certs.sha1Server
	}
	srv := &http.Server{
		Handler:   http.HandlerFunc(s.serveLocal),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{cert}},
		ErrorLog:  quiet,
	}
	go srv.ServeTLS(ln, "", "")

	s.mu.Lock()
	defer s.mu.Unlock()
	s.local = srv
	s.localPort = ln.Addr().(*net.TCPAddr).Port
	s.password = randomToken(16)
	s.lockPass = s.password
	s.issueTokens()
	return s.writeLockfile()
}

func (s *Server) writeLockfile() error {
	content := fmt.Sprintf("Riot Client:%d:%d:%s:https", os.Getpid(), s.localPort, s.lockPass)
	tmp := s.Lockfile() + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.Lockfile())
}

func (s *Server) issueTokens() {
	now := s.now()
	s.tokenExp = now.Add(s.world.TokenLifetime)
	s.entitleExp = s.tokenExp
	if s.world.EntitlementLifetime > 0 {
		s.entitleExp = now.Add(s.world.EntitlementLifetime)
	}
	s.access = makeJWT(s.world.Self, now, s.tokenExp)
	s.entitlement = makeJWT(s.world.Self, now, s.entitleExp)
}

func makeJWT(sub string, iat, exp time.Time) string {
	enc := base64.RawURLEncoding
	header := enc.EncodeToString([]byte(`{"alg":"RS256","kid":"s1"}`))
	payload := enc.EncodeToString(mustJSON(obj{"sub": sub, "iat": iat.Unix(), "exp": exp.Unix(), "jti": randomToken(8)}))
	return header + "." + payload + "." + randomToken(32)
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (s *Server) begin(service, region, shard, path string, r *http.Request) *Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	req := &Request{
		Seq: len(s.log) + 1, Service: service, Region: region, Shard: shard,
		Method: r.Method, Path: path, Query: r.URL.Query(), Header: r.Header.Clone(),
		Start: time.Now(),
	}
	s.log = append(s.log, req)
	if service != "local" {
		s.inflight++
		if s.inflight > s.maxInflight {
			s.maxInflight = s.inflight
		}
	}
	return req
}

func (s *Server) end(req *Request, status int, ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if req.done {
		return
	}
	req.done = true
	req.Status = status
	req.End = time.Now()
	req.Canceled = ctx.Err() != nil
	if req.Service != "local" {
		s.inflight--
	}
}

func (s *Server) matchFault(service, path string) *Fault {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, f := range s.faults {
		if f.Service != "" && f.Service != service {
			continue
		}
		if !strings.HasPrefix(path, f.Path) || f.Exact && path != f.Path {
			continue
		}
		if f.Times > 0 {
			f.Times--
			if f.Times == 0 {
				s.faults = append(s.faults[:i:i], s.faults[i+1:]...)
			}
		}
		return f
	}
	return nil
}

// applyFault reports whether the fault already answered the request
func (s *Server) applyFault(f *Fault, req *Request, w http.ResponseWriter, r *http.Request) bool {
	if f.Delay > 0 {
		select {
		case <-time.After(f.Delay):
		case <-r.Context().Done():
			return true
		}
	}
	if f.Hold {
		h := &heldRequest{req: req, release: make(chan struct{})}
		s.mu.Lock()
		s.held = append(s.held, h)
		s.mu.Unlock()
		select {
		case <-h.release:
		case <-r.Context().Done():
			s.mu.Lock()
			for i, other := range s.held {
				if other == h {
					s.held = append(s.held[:i:i], s.held[i+1:]...)
					break
				}
			}
			s.mu.Unlock()
			return true
		}
	}
	if f.Drop {
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, err := hj.Hijack(); err == nil {
				conn.Close()
				return true
			}
		}
		panic(http.ErrAbortHandler)
	}
	if f.Status == 0 {
		return false
	}
	for k, v := range f.Header {
		w.Header().Set(k, v)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(f.Status)
	w.Write([]byte(f.Body))
	return true
}

func writeJSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	w.Write(body)
}

func (s *Server) serveLocal(rw http.ResponseWriter, r *http.Request) {
	w := &statusWriter{ResponseWriter: rw}
	req := s.begin("local", "", "", r.URL.Path, r)
	defer func() { s.end(req, w.status, r.Context()) }()

	s.mu.Lock()
	password := s.password
	s.mu.Unlock()
	user, pass, ok := r.BasicAuth()
	if !ok || user != "riot" || pass != password {
		writeJSON(w, http.StatusUnauthorized, errorBody(401, "RPC_ERROR", "invalid credentials"))
		return
	}
	if f := s.matchFault("local", r.URL.Path); f != nil && s.applyFault(f, req, w, r) {
		return
	}
	if r.URL.Path == "/" {
		w.status = http.StatusSwitchingProtocols
		s.serveWS(rw, r)
		return
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorBody(405, "RPC_ERROR", "method not allowed"))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	switch r.URL.Path {
	case "/entitlements/v1/token":
		if s.world.TokenNotReady > 0 {
			s.world.TokenNotReady--
			writeJSON(w, http.StatusNotFound, mustJSON(obj{
				"errorCode": "RPC_ERROR", "httpStatus": 404, "implementationDetails": obj{},
				"message": "Entitlements token is not ready yet",
			}))
			return
		}
		if soon := s.now().Add(5 * time.Minute); !soon.Before(s.tokenExp) || !soon.Before(s.entitleExp) {
			s.issueTokens()
		}
		writeJSON(w, http.StatusOK, mustJSON(obj{
			"accessToken": s.access, "entitlements": []any{},
			"issuer": "https://entitlements.auth.riotgames.com", "subject": s.world.Self,
			"token": s.entitlement,
		}))
	case "/chat/v4/presences", "/product-session/v1/external-sessions":
		if raw, ok := s.world.Raw["local "+r.URL.Path]; ok {
			writeJSON(w, raw.Status, raw.Body)
		} else if r.URL.Path == "/chat/v4/presences" {
			writeJSON(w, http.StatusOK, s.world.presences())
		} else {
			writeJSON(w, http.StatusOK, s.world.sessions())
		}
	default:
		writeJSON(w, http.StatusNotFound, errorBody(404, "RESOURCE_NOT_FOUND", "no such endpoint"))
	}
}

func (s *Server) serveRemote(rw http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/fakeriot/requests" {
		writeJSON(rw, http.StatusOK, mustJSON(s.Requests()))
		return
	}
	w := &statusWriter{ResponseWriter: rw}
	parts := splitPath(r.URL.Path)
	var service, region, shard string
	var rest []string
	switch {
	case len(parts) >= 2 && (parts[0] == "pd" || parts[0] == "shared"):
		service, shard, rest = parts[0], parts[1], parts[2:]
	case len(parts) >= 3 && parts[0] == "glz":
		service, region, shard, rest = "glz", parts[1], parts[2], parts[3:]
	case len(parts) >= 1 && parts[0] == "static":
		service, rest = "static", parts[1:]
	default:
		http.NotFound(w, r)
		return
	}
	path := "/" + strings.Join(rest, "/")
	req := s.begin(service, region, shard, path, r)
	defer func() { s.end(req, w.status, r.Context()) }()

	if f := s.matchFault(service, path); f != nil && s.applyFault(f, req, w, r) {
		return
	}
	if service != "static" {
		if status, body := s.checkTokens(r); status != 0 {
			writeJSON(w, status, body)
			return
		}
	}
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusNotFound, errorBody(404, "RESOURCE_NOT_FOUND", "fakeriot only serves reads"))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if raw, ok := s.raw(service, path, r.URL.RawQuery); ok {
		writeJSON(w, raw.Status, raw.Body)
		return
	}
	status, body := s.route(service, path, r.URL.Query())
	writeJSON(w, status, body)
}

func (s *Server) raw(service, path, query string) (RawResponse, bool) {
	if query != "" {
		if raw, ok := s.world.Raw[service+" "+path+"?"+query]; ok {
			return raw, true
		}
	}
	raw, ok := s.world.Raw[service+" "+path]
	return raw, ok
}

func (s *Server) checkTokens(r *http.Request) (int, []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Header.Get("Authorization") != "Bearer "+s.access || r.Header.Get("X-Riot-Entitlements-JWT") != s.entitlement {
		return http.StatusUnauthorized, errorBody(401, "BAD_CLAIMS", "Failure validating/decoding RSO Access Token")
	}
	if !s.now().Before(s.tokenExp) || !s.now().Before(s.entitleExp) {
		return http.StatusBadRequest, errorBody(400, "BAD_CLAIMS", "Failure validating/decoding RSO Access Token")
	}
	return 0, nil
}

func notFound() (int, []byte) {
	return http.StatusNotFound, errorBody(404, "RESOURCE_NOT_FOUND", "resource not found")
}

func intParam(q url.Values, key string, def int) int {
	if v, err := strconv.Atoi(q.Get(key)); err == nil {
		return v
	}
	return def
}

func (s *Server) route(service, path string, q url.Values) (int, []byte) {
	w := s.world
	p := splitPath(path)
	is := func(prefix ...string) bool {
		if len(p) != len(prefix) {
			return false
		}
		for i, part := range prefix {
			if part != "*" && part != p[i] {
				return false
			}
		}
		return true
	}
	switch service {
	case "pd":
		switch {
		case is("mmr", "v1", "players", "*"):
			return http.StatusOK, w.mmr(p[3])
		case is("mmr", "v1", "players", "*", "competitiveupdates"):
			return http.StatusOK, w.competitiveUpdates(p[3], intParam(q, "startIndex", 0), intParam(q, "endIndex", 10))
		case is("match-history", "v1", "history", "*"):
			return http.StatusOK, w.matchHistory(p[3], q.Get("queue"), intParam(q, "startIndex", 0), intParam(q, "endIndex", 20))
		case is("match-details", "v1", "matches", "*"):
			if m, ok := w.Matches[p[3]]; ok {
				return http.StatusOK, w.matchDetails(m)
			}
		}
	case "glz":
		switch {
		case is("pregame", "v1", "players", "*"):
			if body, ok := w.pregamePlayer(p[3]); ok {
				return http.StatusOK, body
			}
		case is("pregame", "v1", "matches", "*"):
			if w.Pregame != nil && w.Pregame.ID == p[3] {
				return http.StatusOK, w.pregameMatch(w.Pregame)
			}
		case is("core-game", "v1", "players", "*"):
			if body, ok := w.coreGamePlayer(p[3]); ok {
				return http.StatusOK, body
			}
		case is("core-game", "v1", "matches", "*"):
			if w.CoreGame != nil && w.CoreGame.ID == p[3] {
				return http.StatusOK, w.coreGameMatch(w.CoreGame)
			}
		case is("parties", "v1", "players", "*"):
			return http.StatusOK, w.partyPlayer(p[3])
		case is("parties", "v1", "parties", "*"):
			if p[3] == w.PartyID {
				return http.StatusOK, w.party()
			}
		}
	case "shared":
		if is("content-service", "v3", "content") {
			return http.StatusOK, w.content()
		}
	case "static":
		switch {
		case is("v1", "version"):
			return http.StatusOK, w.staticVersion()
		case is("v1", "agents"):
			return http.StatusOK, w.staticAgents()
		case is("v1", "maps"):
			return http.StatusOK, w.staticMaps()
		}
	}
	return notFound()
}
