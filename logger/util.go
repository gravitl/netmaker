package logger

import (
	"os"
	"strconv"
	"strings"
)

// MakeString - makes a string using golang string builder
func MakeString(delimeter string, message ...string) string {
	var builder strings.Builder
	for i := 0; i < len(message); i++ {
		builder.WriteString(message[i])
		if delimeter != "" && i != len(message)-1 {
			builder.WriteString(delimeter)
		}
	}
	return builder.String()
}

func getVerbose() int32 {
	level, err := strconv.Atoi(os.Getenv("VERBOSITY"))
	if err != nil || level < 0 {
		level = 0
	}
	if level > 3 {
		level = 3
	}
	return int32(level)
}
