package functions

import (
	"log"
)

func PrintLog(message string, loglevel int) {
	log.SetFlags(log.Flags() &^ (log.Llongfile | log.Lshortfile))
	if loglevel == 0 {
			log.Println(message)
	}
}