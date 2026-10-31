// Package record writes scrubbed recordings that can be shared and replayed as fixtures
package record

import (
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Recorder struct {
	Dir string
	sc  *Scrubber
	now func() time.Time

	mu sync.Mutex
	f  *os.File
}

type entry struct {
	Time    string          `json:"time"`
	Kind    string          `json:"kind"`
	Service string          `json:"service,omitempty"`
	Method  string          `json:"method,omitempty"`
	Path    string          `json:"path,omitempty"`
	Query   string          `json:"query,omitempty"`
	Event   string          `json:"event,omitempty"`
	Status  int             `json:"status,omitempty"`
	Body    json.RawMessage `json:"body"`
}

func Start(parent string, now func() time.Time, public func(string) bool) (*Recorder, error) {
	dir := filepath.Join(parent, "lowkey-record-"+now().Format("20060102-150405"))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, "record.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	return &Recorder{Dir: dir, sc: NewScrubber(public), now: now, f: f}, nil
}

func (r *Recorder) Scrubber() *Scrubber { return r.sc }

func (r *Recorder) write(e entry) {
	e.Time = r.now().Format(time.RFC3339Nano)
	line, err := json.Marshal(e)
	if err != nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f != nil {
		r.f.Write(append(line, '\n'))
	}
}

// Lockfile records the lockfile with its password removed
func (r *Recorder) Lockfile(content, password string) {
	r.sc.Secret(password)
	body, _ := json.Marshal(r.sc.Text(content))
	r.write(entry{Kind: "lockfile", Body: body})
}

func (r *Recorder) Local(path string, status int, body []byte) {
	r.write(entry{Kind: "local", Method: "GET", Path: r.sc.Text(path), Status: status, Body: r.sc.JSON(body)})
}

func (r *Recorder) Remote(service, method, path, query string, status int, body []byte) {
	r.write(entry{
		Kind: "remote", Service: service, Method: method,
		Path: r.sc.Text(path), Query: r.sc.Text(query), Status: status, Body: r.sc.JSON(body),
	})
}

func (r *Recorder) Event(name string, frame []byte) {
	r.write(entry{Kind: "event", Event: name, Body: r.sc.JSON(frame)})
}

// Screen appends a frame as plain text to screens.txt - names must already be aliased
func (r *Recorder) Screen(lines []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, err := os.OpenFile(filepath.Join(r.Dir, "screens.txt"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	f.WriteString("--- " + r.now().Format(time.RFC3339) + "\n" + strings.Join(lines, "\n") + "\n\n")
}

// Chain saves a local API certificate chain that lowkey refused - certificates are public
func (r *Recorder) Chain(chain []*x509.Certificate) {
	var out []byte
	for _, c := range chain {
		out = append(out, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: c.Raw})...)
	}
	os.WriteFile(filepath.Join(r.Dir, "local-api-chain.pem"), out, 0o644)
}

func (r *Recorder) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.f == nil {
		return nil
	}
	err := r.f.Close()
	r.f = nil
	return err
}
