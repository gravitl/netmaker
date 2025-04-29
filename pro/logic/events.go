package logic

import (
	"context"

	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/schema"
)

var EventActivityCh = make(chan schema.Activity, 100)

func LogEvent(a schema.Activity) {
	EventActivityCh <- a
}

func EventWatcher() {

	for e := range EventActivityCh {
		if e.ID == "CLOSE" {
			return
		}
		e.Create(db.WithContext(context.TODO()))
	}

}
