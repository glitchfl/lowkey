package remote

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Service int

const (
	PD Service = iota
	GLZ
	Shared
	Static
)

func (s Service) String() string {
	return [...]string{"pd", "glz", "shared", "static"}[s]
}

// Endpoints are base URL templates with {region} and {shard} placeholders
type Endpoints struct {
	PD     string
	GLZ    string
	Shared string
	Static string
}

func ProductionEndpoints() Endpoints {
	return Endpoints{
		PD:     "https://pd.{shard}.a.pvp.net",
		GLZ:    "https://glz-{region}-1.{shard}.a.pvp.net",
		Shared: "https://shared.{shard}.a.pvp.net",
		Static: "https://valorant-api.com",
	}
}

var clientPlatform = base64.StdEncoding.EncodeToString([]byte(
	`{"platformType":"PC","platformOS":"Windows","platformOSVersion":"10.0.19042.1.256.64bit","platformChipset":"Unknown"}`))

type Tokens struct {
	Access      string
	Entitlement string
	Expires     time.Time // zero when the token carries no exp
}

type StatusError struct {
	Status     int
	RetryAfter time.Duration
	Code       string
}

func (e *StatusError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("HTTP %d %s", e.Status, e.Code)
	}
	return fmt.Sprintf("HTTP %d", e.Status)
}

// UnreachableError means the request never got an answer
type UnreachableError struct {
	Err error
}

func (e *UnreachableError) Error() string { return "unreachable: " + e.Err.Error() }
func (e *UnreachableError) Unwrap() error { return e.Err }

type Config struct {
	Endpoints Endpoints
	Transport http.RoundTripper
	UserAgent string
	Now       func() time.Time
	// Tokens fetches fresh tokens from the Riot Client
	Tokens func(ctx context.Context) (Tokens, error)
	Guard  *Guard
	// Observe sees every answered request - record mode hooks in here
	Observe func(svc Service, path, query string, status int, body []byte)
}

type Client struct {
	cfg  Config
	http *http.Client
	// one refresh at a time - the others wait and reuse its tokens
	refreshing sync.Mutex

	mu      sync.Mutex
	tokens  Tokens
	have    bool
	region  string
	shard   string
	version string
}

func New(cfg Config) *Client {
	tr := cfg.Transport
	if tr == nil {
		tr = http.DefaultTransport
	}
	return &Client{
		cfg: cfg,
		http: &http.Client{
			Transport: tr,
			Timeout:   15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}
}

func (c *Client) SetTarget(region, shard, version string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.region, c.shard, c.version = region, shard, version
}

func (c *Client) Target() (region, shard, version string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.region, c.shard, c.version
}

func (c *Client) URL(svc Service, path string, query url.Values) (*url.URL, error) {
	c.mu.Lock()
	region, shard := c.region, c.shard
	c.mu.Unlock()
	base := map[Service]string{PD: c.cfg.Endpoints.PD, GLZ: c.cfg.Endpoints.GLZ, Shared: c.cfg.Endpoints.Shared, Static: c.cfg.Endpoints.Static}[svc]
	if svc != Static && (region == "" || shard == "") {
		return nil, errors.New("region and shard aren't known yet")
	}
	base = strings.NewReplacer("{region}", region, "{shard}", shard).Replace(base)
	u, err := url.Parse(base + path)
	if err != nil {
		return nil, err
	}
	if len(query) > 0 {
		u.RawQuery = query.Encode()
	}
	return u, nil
}

// Get makes one read, refreshing tokens before they expire and once more if Riot rejects them
func (c *Client) Get(ctx context.Context, svc Service, path string, query url.Values) ([]byte, error) {
	u, err := c.URL(svc, path, query)
	if err != nil {
		return nil, err
	}
	if c.cfg.Guard != nil {
		if err := c.cfg.Guard.Check(http.MethodGet, svc, u); err != nil {
			return nil, err
		}
	}
	body, used, err := c.do(ctx, svc, u, path)
	var se *StatusError
	if svc != Static && errors.As(err, &se) && rejectedToken(se) {
		c.invalidate(used)
		body, _, err = c.do(ctx, svc, u, path)
	}
	return body, err
}

func rejectedToken(se *StatusError) bool {
	return se.Status == http.StatusUnauthorized || se.Status == http.StatusForbidden ||
		(se.Status == http.StatusBadRequest && se.Code == "BAD_CLAIMS")
}

// invalidate forgets the tokens Riot rejected - unless another request already replaced them
func (c *Client) invalidate(used Tokens) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.tokens.Access == used.Access {
		c.have = false
	}
}

func (c *Client) token(ctx context.Context) (Tokens, error) {
	c.refreshing.Lock()
	defer c.refreshing.Unlock()
	c.mu.Lock()
	t, have := c.tokens, c.have
	c.mu.Unlock()
	if have && (t.Expires.IsZero() || c.cfg.Now().Add(2*time.Minute).Before(t.Expires)) {
		return t, nil
	}
	t, err := c.cfg.Tokens(ctx)
	if err != nil {
		return Tokens{}, err
	}
	c.mu.Lock()
	c.tokens, c.have = t, true
	c.mu.Unlock()
	return t, nil
}

func (c *Client) do(ctx context.Context, svc Service, u *url.URL, path string) ([]byte, Tokens, error) {
	var t Tokens
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, t, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)
	if svc != Static {
		if t, err = c.token(ctx); err != nil {
			return nil, t, fmt.Errorf("riot client tokens: %w", err)
		}
		_, _, version := c.Target()
		req.Header.Set("Authorization", "Bearer "+t.Access)
		req.Header.Set("X-Riot-Entitlements-JWT", t.Entitlement)
		req.Header.Set("X-Riot-ClientPlatform", clientPlatform)
		req.Header.Set("X-Riot-ClientVersion", version)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, t, ctx.Err()
		}
		return nil, t, &UnreachableError{Err: stripURL(err)}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		if ctx.Err() != nil {
			return nil, t, ctx.Err()
		}
		return nil, t, &UnreachableError{Err: stripURL(err)}
	}
	if c.cfg.Observe != nil {
		c.cfg.Observe(svc, path, u.RawQuery, resp.StatusCode, body)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, t, &StatusError{Status: resp.StatusCode, RetryAfter: retryAfter(resp.Header), Code: errorCode(body)}
	}
	return body, t, nil
}

// url errors carry the full URL - keep messages short and free of query details
func stripURL(err error) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		return ue.Err
	}
	return err
}

func retryAfter(h http.Header) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}
	return 0
}

func errorCode(body []byte) string {
	var v struct {
		ErrorCode string `json:"errorCode"`
	}
	json.Unmarshal(body, &v)
	return v.ErrorCode
}
