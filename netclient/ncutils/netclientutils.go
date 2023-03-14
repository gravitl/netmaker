package ncutils

import (
	"bytes"
	"crypto/rand"
	"encoding/gob"
)

// DEFAULT_GC_PERCENT - garbage collection percent
const DEFAULT_GC_PERCENT = 10

// == OS PATH FUNCTIONS ==

// ConvertKeyToBytes - util to convert a key to bytes to use elsewhere
func ConvertKeyToBytes(key *[32]byte) ([]byte, error) {
	var buffer bytes.Buffer
	var enc = gob.NewEncoder(&buffer)
	if err := enc.Encode(key); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// ConvertBytesToKey - util to convert bytes to a key to use elsewhere
func ConvertBytesToKey(data []byte) (*[32]byte, error) {
	var buffer = bytes.NewBuffer(data)
	var dec = gob.NewDecoder(buffer)
	var result = new([32]byte)
	var err = dec.Decode(result)
	if err != nil {
		return nil, err
	}
	return result, err
}

// MakeRandomString - generates a random string of len n
func MakeRandomString(n int) string {
	const validChars = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	result := make([]byte, n)
	if _, err := rand.Reader.Read(result); err != nil {
		return ""
	}
	for i, b := range result {
		result[i] = validChars[b%byte(len(validChars))]
	}
	return string(result)
}
