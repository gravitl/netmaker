package mq

import (
	"fmt"

	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/netclient/ncutils"
)

func decryptMsg(msg []byte) ([]byte, error) {
	trafficKey, trafficErr := logic.RetrievePrivateTrafficKey()
	if trafficErr != nil {
		return nil, trafficErr
	}
	return ncutils.DestructMessage(string(msg), &trafficKey), nil
}

func encrypt(node *models.Node, dest string, msg []byte) ([]byte, error) {
	fmt.Printf("original length: %d \n", len(msg))
	node.TrafficKeys.Mine.N = &node.TrafficKeys.Mod
	node.TrafficKeys.Mine.E = node.TrafficKeys.E
	encrypted := ncutils.BuildMessage(msg, &node.TrafficKeys.Mine)
	if encrypted == "" {
		return nil, fmt.Errorf("could not encrypt message")
	}
	fmt.Printf("resulting length: %d \n", len(encrypted))
	return []byte(encrypted), nil
}

func publish(node *models.Node, dest string, msg []byte) error {
	client := SetupMQTT()
	defer client.Disconnect(250)
	encrypted, encryptErr := encrypt(node, dest, msg)
	if encryptErr != nil {
		return encryptErr
	}
	if token := client.Publish(dest, 0, false, encrypted); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	return nil
}
