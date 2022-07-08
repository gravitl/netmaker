package serverctl

import (
	"crypto/ed25519"
	ssl "crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/gravitl/netmaker/database"
	"github.com/gravitl/netmaker/tls"
)

// TlsConfig - holds this servers TLS conf in memory
var TlsConfig ssl.Config

// SaveCert - save a certificate to file and DB
func SaveCert(path, name string, cert *x509.Certificate) error {
	if err := SaveCertToDB(name, cert); err != nil {
		return err
	}
	return tls.SaveCertToFile(path, name, cert)
}

// SaveCertToDB - save a certificate to the certs database
func SaveCertToDB(name string, cert *x509.Certificate) error {
	if certBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}); len(certBytes) > 0 {
		data, err := json.Marshal(&certBytes)
		if err != nil {
			return fmt.Errorf("failed to marshal certificate - %v ", err)
		}
		return database.Insert(name, string(data), database.CERTS_TABLE_NAME)
	} else {
		return fmt.Errorf("failed to write cert to DB - %s ", name)
	}
}

// SaveKey - save a private key (ed25519) to file and DB
func SaveKey(path, name string, key ed25519.PrivateKey) error {
	if err := SaveKeyToDB(name, key); err != nil {
		return err
	}
	return tls.SaveKeyToFile(path, name, key)
}

// SaveKeyToDB - save a private key (ed25519) to the specified path
func SaveKeyToDB(name string, key ed25519.PrivateKey) error {
	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal key %v ", err)
	}
	if pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privBytes,
	}); len(pemBytes) > 0 {
		data, err := json.Marshal(&pemBytes)
		if err != nil {
			return fmt.Errorf("failed to marshal key %v ", err)
		}
		return database.Insert(name, string(data), database.CERTS_TABLE_NAME)
	} else {
		return fmt.Errorf("failed to write key to DB - %v ", err)
	}
}

// ReadCertFromDB - reads a certificate from the database
func ReadCertFromDB(name string) (*x509.Certificate, error) {
	certString, err := database.FetchRecord(database.CERTS_TABLE_NAME, name)
	if err != nil {
		return nil, fmt.Errorf("unable to read file %w", err)
	}
	var certBytes []byte
	if err = json.Unmarshal([]byte(certString), &certBytes); err != nil {
		return nil, fmt.Errorf("unable to unmarshal db cert %w", err)
	}
	block, _ := pem.Decode(certBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("not a cert " + block.Type)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse cert %w", err)
	}
	return cert, nil
}

// ReadKeyFromDB - reads a private key (ed25519) from the database
func ReadKeyFromDB(name string) (*ed25519.PrivateKey, error) {
	keyString, err := database.FetchRecord(database.CERTS_TABLE_NAME, name)
	if err != nil {
		return nil, fmt.Errorf("unable to read key value from db - %w", err)
	}
	var bytes []byte
	if err = json.Unmarshal([]byte(keyString), &bytes); err != nil {
		return nil, fmt.Errorf("unable to unmarshal db key - %w", err)
	}
	keyBytes, _ := pem.Decode(bytes)
	key, err := x509.ParsePKCS8PrivateKey(keyBytes.Bytes)
	if err != nil {
		return nil, fmt.Errorf("unable to parse key from DB -  %w", err)
	}
	private := key.(ed25519.PrivateKey)
	return &private, nil
}

// SetClientTLSConf - saves client cert for servers to connect to MQ broker with
func SetClientTLSConf(serverClientPemPath, serverClientKeyPath string, ca *x509.Certificate) error {
	certpool := x509.NewCertPool()
	if caData := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca.Raw,
	}); len(caData) <= 0 {
		return fmt.Errorf("could not encode CA cert to memory for server client")
	} else {
		ok := certpool.AppendCertsFromPEM(caData)
		if !ok {
			return fmt.Errorf("failed to append root cert to server client cert")
		}
	}
	clientKeyPair, err := ssl.LoadX509KeyPair(serverClientPemPath, serverClientKeyPath)
	if err != nil {
		return err
	}
	certs := []ssl.Certificate{clientKeyPair}

	TlsConfig = ssl.Config{
		RootCAs:            certpool,
		ClientAuth:         ssl.NoClientCert,
		ClientCAs:          nil,
		Certificates:       certs,
		InsecureSkipVerify: false,
	}

	return nil
}
