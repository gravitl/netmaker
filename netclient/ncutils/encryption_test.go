package ncutils

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/nacl/box"
)

func TestBoxDecryptRejectsShortInput(t *testing.T) {
	var pub, priv [32]byte
	_, err := BoxDecrypt(nil, &pub, &priv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")

	_, err = BoxDecrypt([]byte("short"), &pub, &priv)
	require.Error(t, err)
}

func TestDeriveSharedKeySymmetric(t *testing.T) {
	aPub, aPriv, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)
	bPub, bPriv, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)

	keyFromA, err := DeriveSharedKey(aPriv, bPub)
	require.NoError(t, err)
	keyFromB, err := DeriveSharedKey(bPriv, aPub)
	require.NoError(t, err)

	assert.Equal(t, keyFromA, keyFromB, "both sides must derive the identical key")
	assert.Len(t, keyFromA, 32)
}

func TestDeriveSharedKeyDiffersFromRawPublicKey(t *testing.T) {
	aPub, aPriv, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)
	bPub, _, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)

	derived, err := DeriveSharedKey(aPriv, bPub)
	require.NoError(t, err)

	// The whole point of the derivation: the result must not be computable
	// from public keys alone (i.e. it must not just be one of the public
	// keys, which anyone can read via the API).
	assert.NotEqual(t, aPub[:], derived)
	assert.NotEqual(t, bPub[:], derived)
}

func TestDeriveSharedKeyDistinctPeersDiffer(t *testing.T) {
	_, aPriv, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)
	bPub, _, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)
	cPub, _, err := box.GenerateKey(rand.Reader)
	require.NoError(t, err)

	keyWithB, err := DeriveSharedKey(aPriv, bPub)
	require.NoError(t, err)
	keyWithC, err := DeriveSharedKey(aPriv, cPub)
	require.NoError(t, err)

	assert.NotEqual(t, keyWithB, keyWithC)
}
