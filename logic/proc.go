package logic

import (
	"log"
	"os"
	"runtime/pprof"
)

func StartCPUProfiling() *os.File {
	f, err := os.OpenFile("/root/data/cpu.prof", os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		log.Fatal("could not create CPU profile: ", err)
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		log.Fatal("could not start CPU profile: ", err)
	}
	return f
}

func StopCPUProfiling(f *os.File) {
	pprof.StopCPUProfile()
	f.Close()
}
