package mq

import (
	"bytes"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/exp/slog"
)

func decryptMsgWithHost(host *models.Host, msg []byte) ([]byte, error) {
	if host.OS == models.OS_Types.IoT { // just pass along IoT messages
		return msg, nil
	}

	// Check version to determine decryption method
	vlt, err := logic.VersionLessThan(host.Version, "v0.30.0")
	if err != nil {
		slog.Warn("error checking version less than", "error", err)
		// Default to old method if version check fails
		vlt = true
	}

	if vlt {
		// Old decryption method for versions < v0.30.0
		trafficKey, trafficErr := logic.RetrievePrivateTrafficKey() // get server private key
		if trafficErr != nil {
			return nil, trafficErr
		}
		serverPrivTKey, err := ncutils.ConvertBytesToKey(trafficKey)
		if err != nil {
			return nil, err
		}
		nodePubTKey, err := ncutils.ConvertBytesToKey(host.TrafficKeyPublic)
		if err != nil {
			return nil, err
		}

		return ncutils.DeChunk(msg, nodePubTKey, serverPrivTKey)
	} else {
		// New AES-GCM decryption for versions >= v0.30.0
		// For client->server messages, the client encrypts using client private key + server public key
		// The server decrypts using server private key + client public key
		trafficKey, trafficErr := logic.RetrievePrivateTrafficKey()
		if trafficErr != nil {
			return nil, trafficErr
		}
		serverPrivKey, err := ncutils.ConvertBytesToKey(trafficKey)
		if err != nil {
			return nil, err
		}
		clientPubKey, err := ncutils.ConvertBytesToKey(host.TrafficKeyPublic)
		if err != nil {
			return nil, err
		}

		// First decrypt, then decompress
		decrypted, err := decryptAESGCM(serverPrivKey, clientPubKey, msg)
		if err != nil {
			return nil, err
		}
		return decompressPayload(decrypted)
	}
}

func DecryptMsg(node *models.Node, msg []byte) ([]byte, error) {
	if len(msg) <= 24 { // make sure message is of appropriate length
		return nil, fmt.Errorf("received invalid message from broker %v", msg)
	}
	host, err := logic.GetHost(node.HostID.String())
	if err != nil {
		return nil, err
	}

	return decryptMsgWithHost(host, msg)
}

func BatchItems[T any](items []T, batchSize int) [][]T {
	if batchSize <= 0 {
		return nil
	}
	remainderBatchSize := len(items) % batchSize
	nBatches := int(math.Ceil(float64(len(items)) / float64(batchSize)))
	batches := make([][]T, nBatches)
	for i := range batches {
		if i == nBatches-1 && remainderBatchSize > 0 {
			batches[i] = make([]T, remainderBatchSize)
		} else {
			batches[i] = make([]T, batchSize)
		}
		for j := range batches[i] {
			batches[i][j] = items[i*batchSize+j]
		}
	}
	return batches
}

func compressPayload(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		return nil, err
	}
	zw.Close()
	return buf.Bytes(), nil
}

// deriveSharedSecret derives a symmetric key from the server's private key and client's public key
func deriveSharedSecret(serverPrivKey, clientPubKey *[32]byte) []byte {
	// Use NaCl box.Precompute to derive the shared secret
	var sharedSecret [32]byte
	box.Precompute(&sharedSecret, clientPubKey, serverPrivKey)

	// Hash the shared secret to get a 32-byte key for AES
	hash := sha256.Sum256(sharedSecret[:])
	return hash[:]
}

func encryptAESGCM(serverPrivKey, clientPubKey *[32]byte, plaintext []byte) ([]byte, error) {
	// Derive shared secret for symmetric encryption
	key := deriveSharedSecret(serverPrivKey, clientPubKey)

	// Create AES block cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create GCM (Galois/Counter Mode) cipher
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Create a random nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt the data
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func decryptAESGCM(serverPubKey, clientPrivKey *[32]byte, ciphertext []byte) ([]byte, error) {
	// Derive shared secret for symmetric decryption
	key := deriveSharedSecret(clientPrivKey, serverPubKey)

	// Create AES block cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Create GCM cipher
	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Extract nonce from ciphertext
	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Decrypt the data
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func decompressPayload(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func encryptMsg(host *models.Host, msg []byte) ([]byte, error) {
	if host.OS == models.OS_Types.IoT {
		return msg, nil
	}

	// fetch server public key to be certain hasn't changed in transit
	trafficKey, trafficErr := logic.RetrievePrivateTrafficKey()
	if trafficErr != nil {
		return nil, trafficErr
	}

	serverPrivKey, err := ncutils.ConvertBytesToKey(trafficKey)
	if err != nil {
		return nil, err
	}

	nodePubKey, err := ncutils.ConvertBytesToKey(host.TrafficKeyPublic)
	if err != nil {
		return nil, err
	}

	if strings.Contains(host.Version, "0.10.0") {
		return ncutils.BoxEncrypt(msg, nodePubKey, serverPrivKey)
	}

	return ncutils.Chunk(msg, nodePubKey, serverPrivKey)
}

func publish(host *models.Host, dest string, msg []byte) error {

	var encrypted []byte
	var encryptErr error
	vlt, err := logic.VersionLessThan(host.Version, "v0.30.0")
	if err != nil {
		slog.Warn("error checking version less than", "error", err)
		return err
	}
	if vlt {
		encrypted, encryptErr = encryptMsg(host, msg)
		if encryptErr != nil {
			return encryptErr
		}
	} else {
		zipped, err := compressPayload(msg)
		if err != nil {
			return err
		}

		// Get server private key and client public key for AES-GCM encryption
		trafficKey, trafficErr := logic.RetrievePrivateTrafficKey()
		if trafficErr != nil {
			return trafficErr
		}
		serverPrivKey, err := ncutils.ConvertBytesToKey(trafficKey)
		if err != nil {
			return err
		}
		clientPubKey, err := ncutils.ConvertBytesToKey(host.TrafficKeyPublic)
		if err != nil {
			return err
		}

		encrypted, encryptErr = encryptAESGCM(serverPrivKey, clientPubKey, zipped)
		if encryptErr != nil {
			return encryptErr
		}
	}

	if mqclient == nil || !mqclient.IsConnectionOpen() {
		return errors.New("cannot publish ... mqclient not connected")
	}

	if token := mqclient.Publish(dest, 0, true, encrypted); !token.WaitTimeout(MQ_TIMEOUT*time.Second) || token.Error() != nil {
		var err error
		if token.Error() == nil {
			err = errors.New("connection timeout")
		} else {
			slog.Error("publish to mq error", "error", token.Error().Error())
			err = token.Error()
		}
		return err
	}
	return nil
}

// decodes a message queue topic and returns the embedded node.ID
func GetID(topic string) (string, error) {
	parts := strings.Split(topic, "/")
	count := len(parts)
	if count == 1 {
		return "", fmt.Errorf("invalid topic")
	}
	//the last part of the topic will be the node.ID
	return parts[count-1], nil
}
