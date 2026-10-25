package riot

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

//go:embed riotgames.pem
var riotRootPEM []byte

// RiotRoots holds the root that signs the Riot Client's local certificate
func RiotRoots() *x509.CertPool {
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(riotRootPEM) {
		panic("riotgames.pem holds no certificate")
	}
	return pool
}

var ErrTokenNotReady = errors.New("the Riot Client's entitlements token isn't ready yet")

type StatusError struct {
	Status int
	Body   []byte
}

func (e *StatusError) Error() string { return "HTTP " + strconv.Itoa(e.Status) }

// Local talks to the Riot Client's local API - it only ever lives on 127.0.0.1
type Local struct {
	base string
	auth string
	http *http.Client
	tls  *tls.Config
	// Observe sees every response - record mode hooks in here
	Observe func(path string, status int, body []byte)
}

func NewLocal(lf Lockfile, roots *x509.CertPool) *Local {
	conf := &tls.Config{
		// hostname checks can't work for Riot's 127.0.0.1 certificate - verifyLocal checks the chain instead
		InsecureSkipVerify: true,
		VerifyConnection:   verifyLocal(roots),
	}
	tr := &http.Transport{
		Proxy:               nil,
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
		TLSClientConfig:     conf,
		TLSHandshakeTimeout: 5 * time.Second,
		MaxIdleConns:        2,
		IdleConnTimeout:     30 * time.Second,
	}
	return &Local{
		base: "https://127.0.0.1:" + strconv.Itoa(lf.Port),
		auth: "Basic " + base64.StdEncoding.EncodeToString([]byte("riot:"+lf.Password)),
		http: &http.Client{
			Transport: tr,
			Timeout:   10 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		tls: conf,
	}
}

func verifyLocal(roots *x509.CertPool) func(tls.ConnectionState) error {
	return func(cs tls.ConnectionState) error {
		if len(cs.PeerCertificates) == 0 {
			return errors.New("local API sent no certificate")
		}
		leaf := cs.PeerCertificates[0]
		switch leaf.SignatureAlgorithm {
		case x509.SHA1WithRSA, x509.ECDSAWithSHA1, x509.DSAWithSHA1:
			// go can't check SHA-1 signatures any more - the address is 127.0.0.1 and the lockfile password proves the rest
			return nil
		}
		inter := x509.NewCertPool()
		for _, c := range cs.PeerCertificates[1:] {
			inter.AddCert(c)
		}
		if _, err := leaf.Verify(x509.VerifyOptions{Roots: roots, Intermediates: inter}); err != nil {
			return fmt.Errorf("local API certificate from %q (%s) isn't signed by Riot's root: %w",
				leaf.Issuer.CommonName, leaf.SignatureAlgorithm, err)
		}
		return nil
	}
}

func (l *Local) Close() { l.http.CloseIdleConnections() }

func (l *Local) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.base+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", l.auth)
	resp, err := l.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if l.Observe != nil {
		l.Observe(path, resp.StatusCode, body)
	}
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return body, &StatusError{Status: resp.StatusCode, Body: body}
	}
	return body, nil
}

type Entitlements struct {
	AccessToken string `json:"accessToken"`
	Token       string `json:"token"`
	Subject     string `json:"subject"`
}

func (l *Local) Entitlements(ctx context.Context) (Entitlements, error) {
	body, err := l.get(ctx, "/entitlements/v1/token")
	var se *StatusError
	if errors.As(err, &se) && (se.Status == http.StatusNotFound || strings.Contains(strings.ToLower(string(se.Body)), "not ready")) {
		return Entitlements{}, ErrTokenNotReady
	}
	if err != nil {
		return Entitlements{}, err
	}
	var e Entitlements
	if err := json.Unmarshal(body, &e); err != nil {
		return Entitlements{}, fmt.Errorf("entitlements response isn't JSON: %w", err)
	}
	if e.AccessToken == "" || e.Token == "" || e.Subject == "" {
		return Entitlements{}, errors.New("entitlements response is missing a token or the subject")
	}
	return e, nil
}

type Presence struct {
	PUUID   string  `json:"puuid"`
	Product string  `json:"product"`
	Private *string `json:"private"`
}

func (l *Local) Presences(ctx context.Context) ([]Presence, error) {
	body, err := l.get(ctx, "/chat/v4/presences")
	if err != nil {
		return nil, err
	}
	return DecodePresences(body)
}

func DecodePresences(body []byte) ([]Presence, error) {
	var v struct {
		Presences []Presence `json:"presences"`
	}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("presences aren't JSON: %w", err)
	}
	return v.Presences, nil
}

func OwnValorant(list []Presence, self string) (Private, bool) {
	for _, p := range list {
		if !strings.EqualFold(p.PUUID, self) || p.Product != "valorant" || p.Private == nil {
			continue
		}
		priv, err := ParsePrivate(*p.Private)
		if err != nil {
			continue
		}
		return priv, true
	}
	return Private{}, false
}

type Session struct {
	ProductID           string `json:"productId"`
	LaunchConfiguration struct {
		Arguments []string `json:"arguments"`
	} `json:"launchConfiguration"`
}

func (l *Local) Sessions(ctx context.Context) (map[string]Session, error) {
	body, err := l.get(ctx, "/product-session/v1/external-sessions")
	if err != nil {
		return nil, err
	}
	var v map[string]Session
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("external sessions aren't JSON: %w", err)
	}
	return v, nil
}

func ValorantArgs(sessions map[string]Session) ([]string, bool) {
	for _, s := range sessions {
		if s.ProductID == "valorant" {
			return s.LaunchConfiguration.Arguments, true
		}
	}
	return nil, false
}
