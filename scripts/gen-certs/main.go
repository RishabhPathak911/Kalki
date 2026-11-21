// gen-certs creates an offline demo CA and a server certificate with explicit
// DNS/IP SANs. It refuses to overwrite any existing certificate or key.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	dir := flag.String("out", "data/certs", "output directory")
	hosts := flag.String("hosts", "localhost,127.0.0.1,::1", "comma-separated server DNS names and IP addresses")
	flag.Parse()
	for _, name := range []string{"ca.pem", "ca.key", "server.pem", "server.key"} {
		if _, err := os.Lstat(filepath.Join(*dir, name)); err == nil {
			return fmt.Errorf("certificate output already exists; choose another directory")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serverKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	now := time.Now()
	ca := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Kalki LAN Demo CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(1, 0, 0), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, &caKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	leaf := &x509.Certificate{SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "Kalki Setu"}, NotBefore: now.Add(-time.Hour), NotAfter: now.AddDate(0, 3, 0), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	for _, host := range strings.Split(*hosts, ",") {
		host = strings.TrimSpace(host)
		if host == "" {
			return fmt.Errorf("empty certificate host")
		}
		if ip := net.ParseIP(host); ip != nil {
			leaf.IPAddresses = append(leaf.IPAddresses, ip)
		} else {
			leaf.DNSNames = append(leaf.DNSNames, host)
		}
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leaf, ca, &serverKey.PublicKey, caKey)
	if err != nil {
		return err
	}
	caPrivate, err := x509.MarshalECPrivateKey(caKey)
	if err != nil {
		return err
	}
	serverPrivate, err := x509.MarshalECPrivateKey(serverKey)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*dir, 0700); err != nil {
		return err
	}
	for _, entry := range []struct {
		name, kind string
		data       []byte
	}{{"ca.pem", "CERTIFICATE", caDER}, {"ca.key", "EC PRIVATE KEY", caPrivate}, {"server.pem", "CERTIFICATE", leafDER}, {"server.key", "EC PRIVATE KEY", serverPrivate}} {
		f, err := os.OpenFile(filepath.Join(*dir, entry.name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		writeErr := pem.Encode(f, &pem.Block{Type: entry.kind, Bytes: entry.data})
		syncErr := f.Sync()
		closeErr := f.Close()
		if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
			return err
		}
	}
	fmt.Println("Kalki demo CA/server certificates created. Keep .key files local; distribute only ca.pem to collectors.")
	return nil
}
