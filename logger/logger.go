package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

// TimeFormatDay - format of the day for timestamps
const TimeFormatDay = "2006-01-02"

// TimeFormat - total time format
const TimeFormat = "2006-01-02 15:04:05"

// == fields ==
var currentLogs = make(map[string]entry)
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

	if getVerbose() >= 4 {
		pc, file, line, ok := runtime.Caller(1)
		if !ok {
			file = "?"
			line = 0
		}

		fn := runtime.FuncForPC(pc)
		var fnName string
		if fn == nil {
			fnName = "?()"
		} else {
			fnName = strings.TrimLeft(filepath.Ext(fn.Name()), ".") + "()"
		}
		currentMessage = fmt.Sprintf("[%s-%d] %s: %s",
			filepath.Base(file), line, fnName, currentMessage)
	}

	if int32(verbosity) <= getVerbose() && getVerbose() >= 0 {
		fmt.Printf("[%s] %s %s \n", program, currentTime.Format(TimeFormat), currentMessage)
	}

	if program == "netmaker" {
		currentLogs[currentMessage] = entry{
			Time:  currentTime.Format("2006-01-02 15:04:05.999999999"),
			Count: currentLogs[currentMessage].Count + 1,
		}
	}
}

// Dump - dumps all logs into a formatted string
func Dump() string {
	if program != "netmaker" {
		return ""
	}
	mu.Lock()
	defer mu.Unlock()
	var dumpString = ""
	type keyVal struct {
		Key   string
		Value time.Time
		Count int
	}
	var dumpLogs = make([]keyVal, 0, len(currentLogs))
	for key := range currentLogs {
		currentEntry := currentLogs[key]
		parsedTime, err := time.Parse(TimeFormat, currentEntry.Time)
		if err == nil {
			dumpLogs = append(dumpLogs, keyVal{
				Key:   key,
				Value: parsedTime,
				Count: currentEntry.Count,
			})
		}
	}
	sort.Slice(dumpLogs, func(i, j int) bool {
		return dumpLogs[i].Value.Before(dumpLogs[j].Value)
	})

	for i := range dumpLogs {
		var currLog = dumpLogs[i]
		dumpString += MakeString(" ", "[netmaker]", currLog.Value.Format(TimeFormat), currLog.Key, fmt.Sprintf("(%d)", currLog.Count), "\n")
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
	fmt.Printf("[%s] Fatal: %s \n", program, MakeString(" ", message...))
	os.Exit(2)
}

// == private ==

// resetLogs - reallocates logs map
func resetLogs() {
	currentLogs = make(map[string]entry)
}
