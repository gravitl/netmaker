package logic

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/grpc/auditlogs"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/schema"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/datatypes"
)

var EventActivityCh = make(chan models.Event, 100)

var allowUnexported = []any{
	datatypes.JSONType[map[schema.UserGroupID]struct{}]{},
	datatypes.JSONType[schema.ResourceAccess]{},
	datatypes.JSONType[schema.NetworkRoles]{},
}

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
		ID:       "events-retention-hook",
		Hook:     logic.WrapHook(EventRententionHook),
		Interval: time.Hour * 24,
	}
	for e := range EventActivityCh {
		if e.Action == schema.Update {
			// check if diff
			if cmp.Equal(e.Diff.Old, e.Diff.New, cmp.AllowUnexported(allowUnexported...)) {
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

		if GetFeatureFlags().EnableSIEMIntegration {
			sourceMap := make(map[string]interface{})
			dstMap := make(map[string]interface{})
			diffMap := make(map[string]interface{})

			_ = json.Unmarshal(sourceJson, &sourceMap)
			_ = json.Unmarshal(dstJson, &dstMap)
			_ = json.Unmarshal(diff, &diffMap)

			sourceStruct, _ := structpb.NewStruct(sourceMap)
			dstStruct, _ := structpb.NewStruct(dstMap)
			diffStruct, _ := structpb.NewStruct(diffMap)

			_ = auditlogs.Client().Export(&auditlogs.AuditLogEvent{
				Id:          a.ID,
				Action:      string(e.Action),
				Origin:      string(e.Origin),
				TriggeredBy: e.TriggeredBy,
				NetworkId:   string(e.NetworkID),
				Source:      sourceStruct,
				Target:      dstStruct,
				Diff:        diffStruct,
				TsMs:        a.TimeStamp.UnixMilli(),
			})
		}
	}
}
