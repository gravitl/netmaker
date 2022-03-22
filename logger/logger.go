package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// TimeFormatDay - format of the day for timestamps
const TimeFormatDay = "2006-01-02"

// TimeFormat - total time format
const TimeFormat = "2006-01-02 15:04:05"

// == fields ==
var currentLogs = make(map[string]string)
var mu sync.Mutex
var program string

func init() {
	fullpath, err := os.Executable()
	if err != nil {
		fullpath = ""
	}
	program = filepath.Base(fullpath)
}

// Log - handles adding logs
func Log(verbosity int, message ...string) {
	mu.Lock()
	defer mu.Unlock()
	var currentTime = time.Now()
	var currentMessage = MakeString(" ", message...)
	if int32(verbosity) <= getVerbose() && getVerbose() >= 0 {
		fmt.Printf("[%s] %s %s \n", program, currentTime.Format(TimeFormat), currentMessage)
	}
	if program == "netmaker" {
		currentLogs[currentMessage] = currentTime.Format("2006-01-02 15:04:05.999999999")
	}
}

// Dump - dumps all logs into a formatted string
func Dump() string {
	if program != "netmaker" {
		return ""
	}
	var dumpString = ""
	type keyVal struct {
		Key   string
		Value time.Time
	}
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()
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
		dumpString += MakeString(" ", "[netmaker]", currLog.Value.Format(TimeFormat), currLog.Key, "\n")
	}

	resetLogs()
	return dumpString
}

// DumpFile - appends log dump log file
func DumpFile(filePath string) {
	if program != "netmaker" {
		return
	}
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		fmt.Println(MakeString(" ", "could not open log file", filePath))
		return
	}

	defer f.Close()

	if _, err = f.WriteString(Dump()); err != nil {
		fmt.Println("could not dump logs")
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
	var mu sync.Mutex
	mu.Lock()
	defer mu.Unlock()
	fmt.Printf("[netmaker] Fatal: %s \n", MakeString(" ", message...))
	os.Exit(2)
}

// == private ==

// resetLogs - reallocates logs map
func resetLogs() {
	currentLogs = make(map[string]string)
}
