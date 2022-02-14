package ncutils

import (
	"bytes"
	"crypto/rand"
	"encoding/gob"
	"fmt"
	"io"

	"golang.org/x/crypto/nacl/box"
)

const (
	chunkSize = 16128 // 16128 bytes max message size
)

// BoxEncrypt - encrypts traffic box
func BoxEncrypt(message []byte, recipientPubKey *[32]byte, senderPrivateKey *[32]byte) ([]byte, error) {
	var nonce [24]byte // 192 bits of randomization
	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return nil, err
	}

	encrypted := box.Seal(nonce[:], message, &nonce, recipientPubKey, senderPrivateKey)
	return encrypted, nil
}

// BoxDecrypt - decrypts traffic box
func BoxDecrypt(encrypted []byte, senderPublicKey *[32]byte, recipientPrivateKey *[32]byte) ([]byte, error) {
	var decryptNonce [24]byte
	copy(decryptNonce[:], encrypted[:24])
	decrypted, ok := box.Open(nil, encrypted[24:], &decryptNonce, senderPublicKey, recipientPrivateKey)
	if !ok {
		return nil, fmt.Errorf("could not decrypt message")
	}
	return decrypted, nil
}

// Chunk - chunks a message and encrypts each chunk
func Chunk(message []byte, recipientPubKey *[32]byte, senderPrivateKey *[32]byte) ([]byte, error) {
	var chunks [][]byte
	for i := 0; i < len(message); i += chunkSize {
		end := i + chunkSize

		// necessary check to avoid slicing beyond
		// slice capacity
		if end > len(message) {
			end = len(message)
		}

		encryptedMsgSlice, err := BoxEncrypt(message[i:end], recipientPubKey, senderPrivateKey)
		if err != nil {
			return nil, err
		}

		chunks = append(chunks, encryptedMsgSlice)
	}

	chunkedMsg, err := convertBytesToMsg(chunks) // encode the array into some bytes to decode on receiving end
	if err != nil {
		return nil, err
	}

	return chunkedMsg, nil
}

// DeChunk - "de" chunks and decrypts a message
func DeChunk(chunkedMsg []byte, senderPublicKey *[32]byte, recipientPrivateKey *[32]byte) ([]byte, error) {
	chunks, err := convertMsgToBytes(chunkedMsg)
	if err != nil {
		return nil, err
	}

	var totalMsg []byte
	for i := range chunks {
		decodedMsg, err := BoxDecrypt(chunks[i], senderPublicKey, recipientPrivateKey)
		if err != nil {
			return nil, err
		}
		totalMsg = append(totalMsg, decodedMsg...)
	}
	return totalMsg, nil
}

// == private ==

// ConvertMsgToBytes - converts a message (MQ) to it's chunked version
// decode action
func convertMsgToBytes(msg []byte) ([][]byte, error) {
	var buffer = bytes.NewBuffer(msg)
	var dec = gob.NewDecoder(buffer)
	var result [][]byte
	var err = dec.Decode(&result)
	if err != nil {
		return nil, err
	}
	return result, err
}

// ConvertBytesToMsg - converts the chunked message into a MQ message
// encode action
func convertBytesToMsg(b [][]byte) ([]byte, error) {
	var buffer bytes.Buffer
	var enc = gob.NewEncoder(&buffer)
	if err := enc.Encode(b); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
