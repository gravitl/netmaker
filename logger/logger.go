package logger

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const TimeFormatDay = "2006-01-02"
const TimeFormat = "2006-01-02 15:04:05"

var currentLogs = make(map[string]string)

func makeString(message ...string) string {
	return strings.Join(message, " ")
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

// ResetLogs - reallocates logs map
func ResetLogs() {
	currentLogs = make(map[string]string)
}

// Log - handles adding logs
func Log(verbosity int, message ...string) {
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()
	var currentTime = time.Now()
	var currentMessage = makeString(message...)
	if int32(verbosity) <= getVerbose() && getVerbose() >= 0 {
		fmt.Printf("[netmaker] %s %s \n", currentTime.Format(TimeFormat), currentMessage)
	}
	currentLogs[currentMessage] = currentTime.Format("2006-01-02 15:04:05.999999999")
}

// Dump - dumps all logs into a formatted string
func Dump() string {
	var dumpString = ""
	type keyVal struct {
		Key   string
		Value time.Time
	}
	var dumpLogs = make([]keyVal, 0, len(currentLogs))
	for key, value := range currentLogs {
		parsedTime, err := time.Parse(TimeFormat, value)
		if err == nil {
			dumpLogs = append(dumpLogs, keyVal{
				Key:   key,
				Value: parsedTime,
			})
		}
	}
	sort.Slice(dumpLogs, func(i, j int) bool {
		return dumpLogs[i].Value.Before(dumpLogs[j].Value)
	})

	for i := range dumpLogs {
		var currLog = dumpLogs[i]
		dumpString += fmt.Sprintf("[netmaker] %s %s \n", currLog.Value.Format(TimeFormat), currLog.Key)
	}

	return dumpString
}

// DumpFile - appends log dump log file
func DumpFile(filePath string) {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		panic(err)
	}

	defer f.Close()

	if _, err = f.WriteString(Dump()); err != nil {
		panic(err)
	}
}

// Retrieve - retrieves logs from given file
func Retrieve(filePath string) string {
	contents, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}
	return string(contents)
}

// FatalLog - exits os after logging
func FatalLog(message ...string) {
	fmt.Printf("[netmaker] Fatal: %s \n", makeString(message...))
	os.Exit(2)
}
