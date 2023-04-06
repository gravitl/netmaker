package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/turnserver/config"
	"github.com/pion/turn/v2"
)

var (
	authMapLock    = &sync.RWMutex{}
	HostMap        = make(map[string]string)
	authBackUpFile = "auth.json"
)

func init() {
	os.MkdirAll("/etc/config", os.ModePerm)
}

func RegisterNewHostWithTurn(hostID, hostPass string) {
	authMapLock.Lock()
	HostMap[hostID] = string(turn.GenerateAuthKey(hostID, config.GetTurnHost(), hostPass))
	dumpCredsToFile()
	authMapLock.Unlock()
}

func UnRegisterNewHostWithTurn(hostID string) {
	authMapLock.Lock()
	delete(HostMap, hostID)
	dumpCredsToFile()
	authMapLock.Unlock()
}

func dumpCredsToFile() {
	d, err := json.MarshalIndent(HostMap, "", " ")
	if err != nil {
		logger.Log(0, "failed to dump creds to file: ", err.Error())
		return
	}

	err = os.WriteFile(filepath.Join("/etc/config", authBackUpFile), d, os.ModePerm)
	if err != nil {
		logger.Log(0, "failed to backup auth data: ", err.Error())
	}
}
