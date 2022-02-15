package ncutils

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"

	"golang.org/x/crypto/nacl/box"
)

const (
	chunkSize = 16000 // 16000 bytes max message size
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
		return nil, fmt.Errorf("could not decrypt message, %v", encrypted)
	}
	return decrypted, nil
}

// Chunk - chunks a message and encrypts each chunk
func Chunk(message []byte, recipientPubKey *[32]byte, senderPrivateKey *[32]byte) ([]byte, error) {
	var chunks [][]byte
	for i := 0; i < len(message); i += chunkSize {
		end := i + chunkSize

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
	chunks, err := convertMsgToBytes(chunkedMsg) // convert the message to it's original chunks form
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

var splitKey = []byte("|(,)(,)|")

// ConvertMsgToBytes - converts a message (MQ) to it's chunked version
// decode action
func convertMsgToBytes(msg []byte) ([][]byte, error) {
	splitMsg := bytes.Split(msg, splitKey)
	return splitMsg, nil
}

// ConvertBytesToMsg - converts the chunked message into a MQ message
// encode action
func convertBytesToMsg(b [][]byte) ([]byte, error) {

	var buffer []byte  // allocate a buffer with adequate sizing
	for i := range b { // append bytes to it with key
		buffer = append(buffer, b[i]...)
		if i != len(b)-1 {
			buffer = append(buffer, splitKey...)
		}
	}
	return buffer, nil
}
