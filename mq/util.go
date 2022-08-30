package mq

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

func decryptMsg(node *models.Node, msg []byte) ([]byte, error) {
	if len(msg) <= 24 { // make sure message is of appropriate length
		return nil, fmt.Errorf("recieved invalid message from broker %v", msg)
	}

	trafficKey, trafficErr := logic.RetrievePrivateTrafficKey() // get server private key
	if trafficErr != nil {
		return nil, trafficErr
	}
	serverPrivTKey, err := ncutils.ConvertBytesToKey(trafficKey)
	if err != nil {
		return nil, err
	}
	nodePubTKey, err := ncutils.ConvertBytesToKey(node.TrafficKeys.Mine)
	if err != nil {
		return nil, err
	}

	if strings.Contains(node.Version, "0.10.0") {
		return ncutils.BoxDecrypt(msg, nodePubTKey, serverPrivTKey)
	}

	return ncutils.DeChunk(msg, nodePubTKey, serverPrivTKey)
}

func encryptMsg(node *models.Node, msg []byte) ([]byte, error) {
	// fetch server public key to be certain hasn't changed in transit
	trafficKey, trafficErr := logic.RetrievePrivateTrafficKey()
	if trafficErr != nil {
		return nil, trafficErr
	}

	serverPrivKey, err := ncutils.ConvertBytesToKey(trafficKey)
	if err != nil {
		return nil, err
	}

	nodePubKey, err := ncutils.ConvertBytesToKey(node.TrafficKeys.Mine)
	if err != nil {
		return nil, err
	}

	if strings.Contains(node.Version, "0.10.0") {
		return ncutils.BoxEncrypt(msg, nodePubKey, serverPrivKey)
	}

	return ncutils.Chunk(msg, nodePubKey, serverPrivKey)
}

func publish(node *models.Node, dest string, msg []byte) error {
	encrypted, encryptErr := encryptMsg(node, msg)
	if encryptErr != nil {
		return encryptErr
	}
	if mqclient == nil {
		return errors.New("cannot publish ... mqclient not connected")
	}
	if token := mqclient.Publish(dest, 0, true, encrypted); !token.WaitTimeout(MQ_TIMEOUT*time.Second) || token.Error() != nil {
		var err error
		if token.Error() == nil {
			err = errors.New("connection timeout")
		} else {
			err = token.Error()
		}
		return err
	}
	return nil
}

// decodes a message queue topic and returns the embedded node.ID
func getID(topic string) (string, error) {
	parts := strings.Split(topic, "/")
	count := len(parts)
	if count == 1 {
		return "", fmt.Errorf("invalid topic")
	}
	//the last part of the topic will be the node.ID
	return parts[count-1], nil
}
