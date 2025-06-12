package logic

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
)

var EventActivityCh = make(chan models.Event, 100)

func LogEvent(a *models.Event) {
	EventActivityCh <- *a
}

func EventRententionHook() error {
	settings := logic.GetServerSettings()
	retentionPeriod := settings.AuditLogsRetentionPeriodInDays
	if retentionPeriod <= 0 {
		retentionPeriod = 30
	}
	err := (&schema.Event{}).DeleteOldEvents(db.WithContext(context.TODO()), retentionPeriod)
	if err != nil {
		slog.Warn("failed to delete old events pas retention period", "error", err)
	}
	return nil

}

func EventWatcher() {
	logic.HookManagerCh <- models.HookDetails{
		Hook:     EventRententionHook,
		Interval: time.Hour * 24,
	}
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
