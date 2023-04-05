package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/pion/turn/v2"
)

var (
	AuthMapLock    = &sync.RWMutex{}
	HostMap        = make(map[string][]byte)
	authBackUpFile = "auth.json"
)

func RegisterNewHostWithTurn(hostID, hostPass string) {
	AuthMapLock.Lock()
	HostMap[hostID] = turn.GenerateAuthKey(hostID, servercfg.GetTurnHost(), hostPass)
	dumpCredsToFile()
	AuthMapLock.Unlock()
}

func UnRegisterNewHostWithTurn(hostID string) {
	AuthMapLock.Lock()
	delete(HostMap, hostID)
	dumpCredsToFile()
	AuthMapLock.Unlock()
}

func dumpCredsToFile() {
	d, err := json.MarshalIndent(HostMap, "", " ")
	if err != nil {
		logger.Log(0, "failed to dump creds to file: ", err.Error())
		return
	}
	userHomeDir, err := os.UserHomeDir()
	if err != nil {
		logger.Log(0, "failed to get user's home directory")
		return
	}
	err = os.WriteFile(filepath.Join(userHomeDir, authBackUpFile), d, os.ModePerm)
	if err != nil {
		logger.Log(0, "failed to backup auth data: ", userHomeDir, err.Error())
	}
}
