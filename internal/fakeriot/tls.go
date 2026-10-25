package fakeriot

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"sync"
	"time"
)

type certs struct {
	pool   *x509.CertPool
	caPEM  []byte
	server tls.Certificate
	// signed with SHA-1 by a root nobody trusts - the case the spec lets lowkey accept on 127.0.0.1
	sha1Server tls.Certificate
}

var (
	certsOnce sync.Once
	certsVal  certs
	certsErr  error
)

// the Riot Client signs its 127.0.0.1 certificate with its own root - fakeriot does the same
func loadCerts() (certs, error) {
	certsOnce.Do(func() { certsVal, certsErr = makeCerts() })
	return certsVal, certsErr
}

func makeCerts() (certs, error) {
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return certs{}, err
	}
	now := time.Now()
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "fakeriot root"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		return certs{}, err
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		return certs{}, err
	}

	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return certs{}, err
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "127.0.0.1"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1)},
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, ca, &leafKey.PublicKey, caKey)
	if err != nil {
		return certs{}, err
	}

	sha1Server, err := makeSHA1Cert(now)
	if err != nil {
		return certs{}, err
	}

	pool := x509.NewCertPool()
	pool.AddCert(ca)
	return certs{
		pool:       pool,
		caPEM:      pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}),
		server:     tls.Certificate{Certificate: [][]byte{leafDER}, PrivateKey: leafKey},
		sha1Server: sha1Server,
	}, nil
}

func makeSHA1Cert(now time.Time) (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}
	rootTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(3),
		Subject:               pkix.Name{CommonName: "LoL Game Engineering Certificate Authority"},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	root, err := x509.CreateCertificate(rand.Reader, rootTmpl, rootTmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	rootCert, err := x509.ParseCertificate(root)
	if err != nil {
		return tls.Certificate{}, err
	}
	leafTmpl := &x509.Certificate{
		SerialNumber:       big.NewInt(4),
		Subject:            pkix.Name{CommonName: "127.0.0.1"},
		IPAddresses:        []net.IP{net.IPv4(127, 0, 0, 1)},
		NotBefore:          now.Add(-time.Hour),
		NotAfter:           now.Add(10 * 365 * 24 * time.Hour),
		KeyUsage:           x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:        []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		SignatureAlgorithm: x509.SHA1WithRSA,
	}
	leaf, err := x509.CreateCertificate(rand.Reader, leafTmpl, rootCert, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{leaf}, PrivateKey: key}, nil
}
