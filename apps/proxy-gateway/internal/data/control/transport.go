package control

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net/http"
	"os"
)

func NewTransport(caFile string) (*http.Transport, error) {
	if caFile == "" {
		return nil, nil
	}
	raw, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		return nil, err
	}
	if !roots.AppendCertsFromPEM(raw) {
		return nil, errors.New("control CA file contains no certificates")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}
	return transport, nil
}
