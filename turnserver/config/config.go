package config

import (
	"os"
	"strconv"

	"github.com/gravitl/netmaker/config"
)

var (
	Version = "dev"
)

// GetAllowedOrigin - get the allowed origin
func GetAllowedOrigin() string {
	allowedorigin := "*"
	if os.Getenv("CORS_ALLOWED_ORIGIN") != "" {
		allowedorigin = os.Getenv("CORS_ALLOWED_ORIGIN")
	} else if config.Config.Server.AllowedOrigin != "" {
		allowedorigin = config.Config.Server.AllowedOrigin
	}
	return allowedorigin
}

// GetAPIPort - gets the api port
func GetAPIPort() string {
	apiport := "8086"
	if os.Getenv("API_PORT") != "" {
		apiport = os.Getenv("API_PORT")
	} else if config.Config.Server.APIPort != "" {
		apiport = config.Config.Server.APIPort
	}
	return apiport
}

// SetVersion - set version of netmaker
func SetVersion(v string) {
	if v != "" {
		Version = v
	}
}

// GetVersion - version of netmaker
func GetVersion() string {
	return Version
}

// IsDebugMode - gets the debug mode for the server
func IsDebugMode() bool {
	debugMode := false
	if os.Getenv("DEBUG_MODE") == "on" {
		debugMode = true
	}
	return debugMode
}

// GetVerbosity - get logger verbose level
func GetVerbosity() int32 {
	var verbosity = 0
	var err error
	if os.Getenv("VERBOSITY") != "" {
		verbosity, err = strconv.Atoi(os.Getenv("VERBOSITY"))
		if err != nil {
			verbosity = 0
		}
	} else if config.Config.Server.Verbosity != 0 {
		verbosity = int(config.Config.Server.Verbosity)
	}
	if verbosity < 0 || verbosity > 4 {
		verbosity = 0
	}
	return int32(verbosity)
}
