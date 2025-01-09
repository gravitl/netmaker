package logic

import (
	"os"
	"runtime"
	"runtime/pprof"

	"github.com/gravitl/netmaker/logger"
)

func StartCPUProfiling() *os.File {
	f, err := os.OpenFile("/root/data/cpu.prof", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		logger.Log(0, "could not create CPU profile: ", err.Error())
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		logger.Log(0, "could not start CPU profile: ", err.Error())
	}
	return f
}

func StopCPUProfiling(f *os.File) {
	pprof.StopCPUProfile()
	f.Close()
}

func StartMemProfiling() {
	f, err := os.OpenFile("/root/data/mem.prof", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		logger.Log(0, "could not create Memory profile: ", err.Error())
	}
	defer f.Close()
	runtime.GC() // get up-to-date statistics
	if err = pprof.WriteHeapProfile(f); err != nil {
		logger.Log(0, "could not write memory profile: ", err.Error())
	}
}
