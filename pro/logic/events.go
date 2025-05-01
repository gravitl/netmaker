package logic

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

var EventActivityCh = make(chan models.Activity, 100)

func LogEvent(a models.Activity) {
	EventActivityCh <- a
}

func EventWatcher() {

	for e := range EventActivityCh {
		sourceJson, _ := json.Marshal(e.Source)
		dstJson, _ := json.Marshal(e.Target)
		a := schema.Event{
			ID:        uuid.New().String(),
			Action:    e.Action,
			Source:    sourceJson,
			Target:    dstJson,
			Origin:    e.Origin,
			NetworkID: e.NetworkID,
			TimeStamp: time.Now().UTC(),
		}
		a.Create(db.WithContext(context.TODO()))
	}

}
