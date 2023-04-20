package auth

import (
	"encoding/base64"
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
	backUpFilePath = filepath.Join("/etc/config", authBackUpFile)
)

func init() {
	os.MkdirAll("/etc/config", os.ModePerm)
	loadCredsFromFile()
}

func RegisterNewHostWithTurn(hostID, hostPass string) {
	authMapLock.Lock()
	HostMap[hostID] = base64.StdEncoding.EncodeToString(turn.GenerateAuthKey(hostID, config.GetTurnHost(), hostPass))
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

	err = os.WriteFile(backUpFilePath, d, os.ModePerm)
	if err != nil {
		logger.Log(0, "failed to backup auth data: ", err.Error())
	}
}

func loadCredsFromFile() error {
	d, err := os.ReadFile(backUpFilePath)
	if err != nil {
		return err
	}
	return json.Unmarshal(d, &HostMap)
}
