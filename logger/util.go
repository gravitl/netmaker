package logger

import (
	"os"
	"strconv"
	"strings"
)

// Verbosity - current logging verbosity level (optionally set)
var Verbosity = 0

// MakeString - makes a string using golang string builder
func MakeString(delimeter string, message ...string) string {
	var builder strings.Builder
	for i := range message {
		builder.WriteString(message[i])
		if delimeter != "" && i != len(message)-1 {
			builder.WriteString(delimeter)
		}
	}
	return builder.String()
}

func getVerbose() int32 {
	if Verbosity >= 1 && Verbosity <= 3 {
		return int32(Verbosity)
	}
	level, err := strconv.Atoi(os.Getenv("VERBOSITY"))
	if err != nil || level < 0 {
		level = 0
	}
	if level > 3 {
		level = 3
	}
	return int32(level)
}
