package config

import (
	"os"
	"strconv"
)

var (
	Version = "dev"
)

// GetAllowedOrigin - get the allowed origin
func GetAllowedOrigin() string {
	allowedorigin := "*"
	if os.Getenv("CORS_ALLOWED_ORIGIN") != "" {
		allowedorigin = os.Getenv("CORS_ALLOWED_ORIGIN")
	}
	return allowedorigin
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
	}
	if verbosity < 0 || verbosity > 4 {
		verbosity = 0
	}
	return int32(verbosity)
}

// GetTurnHost - fetches the turn host name
func GetTurnHost() string {
	turnServer := ""
	if os.Getenv("TURN_SERVER_HOST") != "" {
		turnServer = os.Getenv("TURN_SERVER_HOST")
	}
	return turnServer
}

// GetTurnPort - Get the port to run the turn server on
func GetTurnPort() int {
	port := 3479 //default
	if os.Getenv("TURN_PORT") != "" {
		portInt, err := strconv.Atoi(os.Getenv("TURN_PORT"))
		if err == nil {
			port = portInt
		}
	}
	return port
}

// GetAPIPort - gets the api port
func GetAPIPort() int {
	apiport := 8089
	if os.Getenv("TURN_API_PORT") != "" {
		portInt, err := strconv.Atoi(os.Getenv("TURN_API_PORT"))
		if err == nil {
			apiport = portInt
		}
	}
	return apiport
}
