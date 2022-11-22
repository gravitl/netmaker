package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Context struct {
	Endpoint  string `yaml:"endpoint"`
	Username  string `yaml:"username,omitempty"`
	Password  string `yaml:"password,omitempty"`
	MasterKey string `yaml:"masterkey,omitempty"`
	Current   bool   `yaml:"current,omitempty"`
}

var (
	contextMap     = map[string]Context{}
	configFilePath string
	filename       string
)

func createConfigPathIfNotExists() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	configFilePath = filepath.Join(homeDir, ".netmaker")
	// create directory if not exists
	if err := os.MkdirAll(configFilePath, os.ModePerm); err != nil {
		log.Fatal(err)
	}
	filename = filepath.Join(configFilePath, "config.yml")
	// create file if not exists
	if _, err := os.Stat(filename); err != nil {
		if os.IsNotExist(err) {
			if _, err := os.Create(filename); err != nil {
				log.Fatalf("Unable to create file filename: %s", err)
			}
		} else {
			log.Fatal(err)
		}
	}
}

func loadConfig() {
	content, err := os.ReadFile(filename)
	if err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}
	if err := yaml.Unmarshal(content, &contextMap); err != nil {
		log.Fatalf("Unable to decode YAML into struct: %s", err)
	}
}

func GetCurrentContext() (ret Context) {
	for _, ctx := range contextMap {
		if ctx.Current {
			ret = ctx
			return
		}
	}
	log.Fatalf("No current context set, do so via `netmaker context use <name>`")
	return
}

func SaveContext() {
	bodyBytes, err := yaml.Marshal(&contextMap)
	if err != nil {
		log.Fatalf("Error marshalling into YAML %s", err)
	}
	file, err := os.Create(filename)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := file.Write(bodyBytes); err != nil {
		log.Fatal(err)
	}
	if err := file.Close(); err != nil {
		log.Fatal(err)
	}
}

func SetCurrentContext(ctxName string) {
	if _, ok := contextMap[ctxName]; !ok {
		log.Fatalf("No such context %s", ctxName)
	}
	for key, ctx := range contextMap {
		ctx.Current = key == ctxName
		contextMap[key] = ctx
	}
	SaveContext()
}

func SetContext(ctxName string, ctx Context) {
	if oldCtx, ok := contextMap[ctxName]; ok && oldCtx.Current {
		ctx.Current = true
	}
	contextMap[ctxName] = ctx
	SaveContext()
}

func DeleteContext(ctxName string) {
	if _, ok := contextMap[ctxName]; ok {
		delete(contextMap, ctxName)
		SaveContext()
	} else {
		log.Fatalf("No such context %s", ctxName)
	}
}

func ListAll() {
	for key, ctx := range contextMap {
		fmt.Print("\n", key, " -> ", ctx.Endpoint)
		if ctx.Current {
			fmt.Print(" (current)")
		}
	}
}

func init() {
	createConfigPathIfNotExists()
	loadConfig()
}
