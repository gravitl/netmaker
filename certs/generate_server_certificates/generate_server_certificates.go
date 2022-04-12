package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log"
	"os"

	"github.com/gravitl/netmaker/tls"
)

// generate root ca/key and server certificate/key for use with mq
func main() {
	if len(os.Args) < 2 {
		fmt.Printf("usage %s: server-name(fqdn) or IP address\n", os.Args[0])
		os.Exit(1)
	}
	server := os.Args[1]

	caName := tls.NewName("CA Root", "US", "Gravitl")
	serverName := tls.NewCName(server)
	_, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal("generate server key ", err)
	}
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatal("generate root key ", err)
	}
	csr, err := tls.NewCSR(key, caName)
	if err != nil {
		log.Fatal("generate root request ", err)
	}
	serverCSR, err := tls.NewCSR(sk, serverName)
	if err != nil {
		log.Fatal("generate server request ", err)
	}
	rootCA, err := tls.SelfSignedCA(key, csr, 365)
	if err != nil {
		log.Fatal("generate root ca ", err)
	}
	serverCert, err := tls.NewEndEntityCert(key, serverCSR, rootCA, 365)
	if err != nil {
		log.Fatal("generate server certificate", err)
	}
	err = tls.SaveCert("./certs/", "server.pem", serverCert)
	if err != nil {
		log.Fatal("save server certificate", err)
	}
	err = tls.SaveCert("./certs/", "root.pem", rootCA)
	if err != nil {
		log.Fatal("save root ca ", err)
	}
	err = tls.SaveKey("./certs/", "root.key", sk)
	if err != nil {
		log.Fatal("save root key ", err)
	}
	err = tls.SaveKey("./certs/", "server.key", sk)
	if err != nil {
		log.Fatal("save server key", err)
	}

}
