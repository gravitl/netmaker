package logic

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

var EventActivityCh = make(chan models.Event, 100)

func LogEvent(a *models.Event) {
	EventActivityCh <- *a
}

func EventWatcher() {

	for e := range EventActivityCh {
		if e.Action == models.Update {
			// check if diff
			if cmp.Equal(e.Diff.Old, e.Diff.New) {
				continue
			}
		}
		sourceJson, _ := json.Marshal(e.Source)
		dstJson, _ := json.Marshal(e.Target)
		diff, _ := json.Marshal(e.Diff)
		a := schema.Event{
			ID:          uuid.New().String(),
			Action:      e.Action,
			Source:      sourceJson,
			Target:      dstJson,
			Origin:      e.Origin,
			NetworkID:   e.NetworkID,
			TriggeredBy: e.TriggeredBy,
			Diff:        diff,
			TimeStamp:   time.Now().UTC(),
		}
		a.Create(db.WithContext(context.TODO()))
	}

}
