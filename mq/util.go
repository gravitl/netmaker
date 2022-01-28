package mq

import (
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

func decryptMsg(nodeid string, msg []byte) ([]byte, error) {
	trafficKey, trafficErr := logic.RetrieveTrafficKey(nodeid)
	if trafficErr != nil {
		return nil, trafficErr
	}
	return ncutils.DecryptWithPrivateKey(msg, &trafficKey), nil
}

func encrypt(nodeid string, dest string, msg []byte) ([]byte, error) {
	var node, err = logic.GetNodeByID(nodeid)
	if err != nil {
		return nil, err
	}
	encrypted, encryptErr := ncutils.EncryptWithPublicKey(msg, &node.TrafficKeys.Mine)
	if encryptErr != nil {
		return nil, encryptErr
	}
	return encrypted, nil
}

func publish(nodeid string, dest string, msg []byte) error {
	client := SetupMQTT()
	defer client.Disconnect(250)
	encrypted, encryptErr := encrypt(nodeid, dest, msg)
	if encryptErr != nil {
		return encryptErr
	}
	if token := client.Publish(dest, 0, false, encrypted); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}
