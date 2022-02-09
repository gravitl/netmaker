package mq

import (
	"fmt"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

func decryptMsg(node *models.Node, msg []byte) ([]byte, error) {
	if len(msg) <= 24 { // make sure message is of appropriate length
		return nil, fmt.Errorf("recieved invalid message from broker %s", string(msg))
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

	return ncutils.BoxDecrypt(msg, nodePubTKey, serverPrivTKey)
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

	return ncutils.BoxEncrypt(msg, nodePubKey, serverPrivKey)
}

func publish(node *models.Node, dest string, msg []byte) error {
	client := SetupMQTT(true)
	defer client.Disconnect(250)
	encrypted, encryptErr := encryptMsg(node, msg)
	if encryptErr != nil {
		return encryptErr
	}
	if token := client.Publish(dest, 0, true, encrypted); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}
