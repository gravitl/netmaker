package ssl

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"log"
	"math/big"
	"os"
	"time"
)

// creates a new pkix.Name
func NewName(commonName, country, org string) pkix.Name {
	res := NewCName(commonName)
	res.Country = []string{country}
	res.Organization = []string{org}
	return res
}

// creates a new pkix.Name with only a common name
func NewCName(commonName string) pkix.Name {
	return pkix.Name{
		CommonName: commonName,
	}
}

// creates a new certificate signing request for a
func NewCSR(key ed25519.PrivateKey, name pkix.Name) (*x509.CertificateRequest, error) {
	derCertRequest, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject:   name,
		PublicKey: key.Public(),
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

// returns a new self-signed certificate
func SelfSignedCA(key ed25519.PrivateKey, req *x509.CertificateRequest, days int) (*x509.Certificate, error) {

	template := &x509.Certificate{
		BasicConstraintsValid: true,
		IsCA:                  true,
		Version:               req.Version,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
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

// issues a new certificate from a parent certificate authority
func NewEndEntityCert(key ed25519.PrivateKey, req *x509.CertificateRequest, parent *x509.Certificate, days int) (*x509.Certificate, error) {
	template := &x509.Certificate{
		Version:            req.Version,
		NotBefore:          time.Now(),
		NotAfter:           time.Now().Add(duration(days)),
		SerialNumber:       serialNumber(),
		SignatureAlgorithm: req.SignatureAlgorithm,
		PublicKeyAlgorithm: req.PublicKeyAlgorithm,
		PublicKey:          req.PublicKey,
		Subject:            req.Subject,
		SubjectKeyId:       req.RawSubject,
		Issuer:             parent.Subject,
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

func SaveCert(name string, cert *x509.Certificate) error {
	//certbytes, err := x509.ParseCertificate(cert)
	certOut, err := os.Create(name + ".PEM")
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

func SaveKey(name string, key *ed25519.PrivateKey) error {
	//func SaveKey(name string, key *ecdsa.PrivateKey) error {
	keyOut, err := os.Create(name + ".key")
	if err != nil {
		return fmt.Errorf("failed open key file for writing: %v", err)
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(*key)
	if err != nil {
		return fmt.Errorf("failedto marshal key %v ", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	}); err != nil {
		return fmt.Errorf("failed to write key to file %v", err)
	}
	pubOut, err := os.Create(name + ".pub")
	if err != nil {
		return fmt.Errorf("failed open key file for writing: %v", err)
	}
	defer pubOut.Close()
	pubBytes, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return fmt.Errorf("failedto marshal key %v ", err)
	}
	if err := pem.Encode(pubOut, &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	}); err != nil {
		return fmt.Errorf("failed to write key to file %v", err)
	}

	return nil
}

func ReadCert(name string) (*x509.Certificate, error) {
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

func ReadKey(name string) (*ed25519.PrivateKey, error) {
	bytes, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %w", err)
	}
	keyBytes, _ := pem.Decode(bytes)
	log.Println(keyBytes.Type)
	key, err := x509.ParsePKCS8PrivateKey(keyBytes.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse file %w", err)
	}
	private := key.(ed25519.PrivateKey)
	return &private, nil
}

func serialNumber() *big.Int {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil
	}
	return serialNumber
}

func duration(days int) time.Duration {
	hours := days * 24
	duration, err := time.ParseDuration(fmt.Sprintf("%dh", hours))
	if err != nil {
		duration = time.Until(time.Now().Add(time.Hour * 24))
	}
	return duration
}
