package ncutils

import (
	"fmt"
	"strings"
	"time"
)

// BackOff - back off any function while there is an error
func BackOff(isExponential bool, maxTime int, f interface{}) (interface{}, error) {
	// maxTime seconds
	startTime := time.Now()
	sleepTime := time.Second
	for time.Now().Before(startTime.Add(time.Second * time.Duration(maxTime))) {
		if result, err := f.(func() (interface{}, error))(); err == nil {
			return result, nil
		}
		time.Sleep(sleepTime)
		if isExponential {
			sleepTime = sleepTime << 1
		}
		PrintLog("retrying...", 1)
	}
	return nil, fmt.Errorf("could not find result")
}

// DestructMessage - reconstruct original message through chunks
func DestructMessage(builtMsg string, senderPublicKey *[32]byte, recipientPrivateKey *[32]byte) ([]byte, error) {
	var chunks = strings.Split(builtMsg, splitKey)
	var totalMessage = make([]byte, len(builtMsg))
	for _, chunk := range chunks {
		var bytes, decErr = BoxDecrypt([]byte(chunk), senderPublicKey, recipientPrivateKey)
		if decErr != nil || bytes == nil {
			return nil, decErr
		}
		totalMessage = append(totalMessage, bytes...)
	}
	return totalMessage, nil
}

// BuildMessage Build a message for publishing
func BuildMessage(originalMessage []byte, recipientPubKey *[32]byte, senderPrivateKey *[32]byte) (string, error) {
	chunks := getSliceChunks(originalMessage, 16128)
	var sb strings.Builder
	for i := 0; i < len(chunks); i++ {
		var encryptedText, encryptErr = BoxEncrypt(chunks[i], recipientPubKey, senderPrivateKey)
		if encryptErr != nil {
			return "", encryptErr
		}
		sb.Write(encryptedText)
		if i < len(chunks)-1 {
			sb.WriteString(splitKey)
		}
	}
	return sb.String(), nil
}

var splitKey = "<|#|>"

func getSliceChunks(slice []byte, chunkSize int) [][]byte {
	var chunks [][]byte
	for i := 0; i < len(slice); i += chunkSize {
		lastByte := i + chunkSize

		if lastByte > len(slice) {
			lastByte = len(slice)
		}

		chunks = append(chunks, slice[i:lastByte])
	}

	return chunks
}
