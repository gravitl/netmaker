package ncutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBoxDecryptRejectsShortInput(t *testing.T) {
	var pub, priv [32]byte
	_, err := BoxDecrypt(nil, &pub, &priv)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")

	_, err = BoxDecrypt([]byte("short"), &pub, &priv)
	require.Error(t, err)
}
