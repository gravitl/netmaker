package models

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"

	"filippo.io/edwards25519"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type (
	Key struct {
		point *edwards25519.Point
	}
)

// Generates a new key.
func NewKey() (*Key, error) {
	seed := make([]byte, 64)
	rand.Reader.Read(seed)
	s, err := (&edwards25519.Scalar{}).SetUniformBytes(seed)
	if err != nil {
		return nil, err
	}
	return &Key{(&edwards25519.Point{}).ScalarBaseMult(s)}, nil
}

// Returns the private key in Edwards form used for EdDSA.
func (n *Key) Ed25519PrivateKey() (ed25519.PrivateKey, error) {
	if n.point == nil {
		return ed25519.PrivateKey{}, errors.New("nil point")
	}
	if len(n.point.Bytes()) != ed25519.SeedSize {
		return ed25519.PrivateKey{}, errors.New("incorrect seed size")
	}
	return ed25519.NewKeyFromSeed(n.point.Bytes()), nil
}

// Returns the private key in Montogomery form used for ECDH.
func (n *Key) Curve25519PrivateKey() (wgtypes.Key, error) {
	if n.point == nil {
		return wgtypes.Key{}, errors.New("nil point")
	}
	if len(n.point.Bytes()) != ed25519.SeedSize {
		return wgtypes.Key{}, errors.New("incorrect seed size")
	}
	return wgtypes.ParseKey(base64.StdEncoding.EncodeToString(n.point.BytesMontgomery()))
}

// Saves the private key to path.
func (n *Key) Save(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	f.Write(n.point.Bytes())
	return nil
}

// Reads the private key from path.
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
