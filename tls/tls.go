package tls

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"time"

	"filippo.io/edwards25519"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	// CERTTIFICATE_VALIDITY duration of certificate validity in days
	CERTIFICATE_VALIDITY = 365

	// SERVER_KEY_NAME - name of server cert private key
	SERVER_KEY_NAME = "server.key"

	// ROOT_KEY_NAME - name of root cert private key
	ROOT_KEY_NAME = "root.key"

	// SERVER_PEM_NAME - name of server pem
	SERVER_PEM_NAME = "server.pem"

	// ROOT_PEM_NAME - name of root pem
	ROOT_PEM_NAME = "root.pem"

	// SERVER_CLIENT_PEM - the name of server client cert
	SERVER_CLIENT_PEM = "serverclient.pem"

	// SERVER_CLIENT_KEY - the name of server client key
	SERVER_CLIENT_KEY = "serverclient.key"

	// SERVER_CLIENT_ENTRY - the server client cert key for DB
	SERVER_CLIENT_ENTRY = "servercliententry"
)

type (
	// Key is the struct for an edwards representation point
	Key struct {
		point *edwards25519.Point
	}
)

// NewKey generates a new key.
func NewKey() *Key {
	seed := make([]byte, 64)
	rand.Reader.Read(seed)
	s, _ := (&edwards25519.Scalar{}).SetUniformBytes(seed)
	return &Key{(&edwards25519.Point{}).ScalarBaseMult(s)}
}

// Key.Ed25519PrivateKey returns the private key in Edwards form used for EdDSA.
func (n *Key) Ed25519PrivateKey() (ed25519.PrivateKey, error) {
	if n.point == nil {
		return ed25519.PrivateKey{}, errors.New("nil point")
	}
	if len(n.point.Bytes()) != ed25519.SeedSize {
		return ed25519.PrivateKey{}, errors.New("incorrect seed size")
	}
	return ed25519.NewKeyFromSeed(n.point.Bytes()), nil
}

// Key.Curve25519PrivateKey returns the private key in Montogomery form used for ECDH.
func (n *Key) Curve25519PrivateKey() (wgtypes.Key, error) {
	if n.point == nil {
		return wgtypes.Key{}, errors.New("nil point")
	}
	if len(n.point.Bytes()) != ed25519.SeedSize {
		return wgtypes.Key{}, errors.New("incorrect seed size")
	}
	return wgtypes.ParseKey(base64.StdEncoding.EncodeToString(n.point.BytesMontgomery()))
}

// Key.Save : saves the private key to path.
func (n *Key) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	f.Write(n.point.Bytes())
	return nil
}

// ReadFrom reads a private key from path.
func ReadFrom(path string) (*Key, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	point, err := (&edwards25519.Point{}).SetBytes(key)
	if err != nil {
		return nil, err
	}
	return &Key{point}, nil
}

// NewName creates a new pkix.Name with common name, country, and organization
func NewName(commonName, country, org string) pkix.Name {
	res := NewCName(commonName)
	res.Country = []string{country}
	res.Organization = []string{org}
	return res
}

// NewCName creates a new pkix.Name with only a common name
func NewCName(commonName string) pkix.Name {
	return pkix.Name{
		CommonName: commonName,
	}
}

// NewCSR creates a new certificate signing request for a
func NewCSR(key ed25519.PrivateKey, name pkix.Name) (*x509.CertificateRequest, error) {
	dnsnames := []string{}
	dnsnames = append(dnsnames, name.CommonName)
	derCertRequest, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:            name,
		PublicKey:          key.Public(),
		DNSNames:           dnsnames,
		PublicKeyAlgorithm: x509.Ed25519,
		Version:            3,
	}, key)
	if err != nil {
		return nil, err
	}
	csr, err := x509.ParseCertificateRequest(derCertRequest)
	if err != nil {
		return nil, err
	}
	return csr, nil
}

// SelfSignedCA returns a new self-signed certificate
func SelfSignedCA(key ed25519.PrivateKey, req *x509.CertificateRequest, days int) (*x509.Certificate, error) {

	template := &x509.Certificate{
		BasicConstraintsValid: true,
		IsCA:                  true,
		Version:               req.Version,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDataEncipherment,
		NotAfter:              time.Now().Add(duration(days)),
		NotBefore:             time.Now(),
		SerialNumber:          serialNumber(),
		PublicKey:             key.Public(),
		Subject: pkix.Name{
			CommonName:   req.Subject.CommonName,
			Organization: req.Subject.Organization,
			Country:      req.Subject.Country,
		},
	}
	rootCa, err := x509.CreateCertificate(rand.Reader, template, template, req.PublicKey, key)
	if err != nil {
		return nil, err
	}
	result, err := x509.ParseCertificate(rootCa)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// NewEndEntityCert issues a new certificate from a parent certificate authority
func NewEndEntityCert(key ed25519.PrivateKey, req *x509.CertificateRequest, parent *x509.Certificate, days int) (*x509.Certificate, error) {
	template := &x509.Certificate{
		Version:               req.Version,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(duration(days)),
		SerialNumber:          serialNumber(),
		Subject:               req.Subject,
		Issuer:                parent.Subject,
		KeyUsage:              x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		DNSNames:              req.DNSNames,
	}
	rootCa, err := x509.CreateCertificate(rand.Reader, template, parent, req.PublicKey, key)
	if err != nil {
		return nil, err
	}
	result, err := x509.ParseCertificate(rootCa)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// SaveRequest saves a certificate request to the specified path
func SaveRequest(path, name string, csr *x509.CertificateRequest) error {
	if err := os.MkdirAll(path, 0600); err != nil {
		return err
	}
	requestOut, err := os.Create(path + name)
	if err != nil {
		return err
	}
	defer requestOut.Close()
	if err := pem.Encode(requestOut, &pem.Block{
		Type:  "CERTIFICATE REQUEST",
		Bytes: csr.Raw,
	}); err != nil {
		return err
	}
	return nil
}

// SaveCertToFile save a certificate to the specified path
func SaveCertToFile(path, name string, cert *x509.Certificate) error {
	//certbytes, err := x509.ParseCertificate(cert)
	if err := os.MkdirAll(path, 0600); err != nil {
		return fmt.Errorf("failed to create dir %s %w", path, err)
	}
	certOut, err := os.Create(path + name)
	if err != nil {
		return fmt.Errorf("failed to open certficate file for writing: %v", err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}); err != nil {
		return fmt.Errorf("failed to write certificate to file %v", err)
	}
	return nil
}

// SaveKeyToFile save a private key (ed25519) to the certs database
func SaveKeyToFile(path, name string, key ed25519.PrivateKey) error {
	//func SaveKey(name string, key *ecdsa.PrivateKey) error {
	if err := os.MkdirAll(path, 0600); err != nil {
		return fmt.Errorf("failed to create dir %s %w", path, err)
	}
	keyOut, err := os.Create(path + name)
	if err != nil {
		return fmt.Errorf("failed open key file for writing: %v", err)
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("failedto marshal key %v ", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	}); err != nil {
		return fmt.Errorf("failed to write key to file %v", err)
	}
	return nil
}

// ReadCertFromFile reads a certificate from disk
func ReadCertFromFile(name string) (*x509.Certificate, error) {
	contents, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %w", err)
	}
	block, _ := pem.Decode(contents)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("not a cert " + block.Type)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse cert %w", err)
	}
	return cert, nil
}

// ReadKeyFromFile reads a private key (ed25519) from disk
func ReadKeyFromFile(name string) (*ed25519.PrivateKey, error) {
	bytes, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %w", err)
	}
	keyBytes, _ := pem.Decode(bytes)
	key, err := x509.ParsePKCS8PrivateKey(keyBytes.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse file %w", err)
	}
	private := key.(ed25519.PrivateKey)
	return &private, nil
}

// serialNumber generates a serial number for a certificate
func serialNumber() *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil
	}
	return serialNumber
}

// duration coverts the number of days to time.duration
func duration(days int) time.Duration {
	hours := days * 24
	duration, err := time.ParseDuration(fmt.Sprintf("%dh", hours))
	if err != nil {
		duration = time.Until(time.Now().Add(time.Hour * 24))
	}
	return duration
}
