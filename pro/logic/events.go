package logic

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/uuid"
	"github.com/gravitl/netmaker/db"
	"github.com/gravitl/netmaker/grpc/auditlogs"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
	"github.com/gravitl/netmaker/pro/integration"
	"github.com/gravitl/netmaker/schema"
	"github.com/gravitl/netmaker/scope"
	"google.golang.org/protobuf/types/known/structpb"
	"gorm.io/datatypes"
)

var EventActivityCh = make(chan models.Event, 100)

var allowUnexported = []any{
	datatypes.JSONType[map[schema.UserGroupID]struct{}]{},
	datatypes.JSONType[schema.ResourceAccess]{},
	datatypes.JSONType[schema.NetworkRoles]{},
}

var _siemMtx sync.Mutex
var _pushToSiem bool

func LogEvent(ctx context.Context, a *models.Event) {
	a.TenantID = scope.ID(ctx)
	EventActivityCh <- *a
}

func EventRetentionHook() error {
	org, err := logic.SoleOrganization(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	orgCtx := scope.WithContext(db.WithContext(context.TODO()), scope.OrgScope, org.ID)
	retentionPeriod := logic.GetOrgSettings(orgCtx).AuditLogsRetentionPeriodInDays
	if retentionPeriod <= 0 {
		retentionPeriod = 30
	}

	tenants, err := (&schema.Tenant{}).List(db.WithContext(context.TODO()))
	if err != nil {
		return err
	}

	for _, tenant := range tenants {
		tenantCtx := scope.WithContext(db.WithContext(context.TODO()), scope.TenantScope, tenant.ID)
		if err := (&schema.Event{}).DeleteOldEvents(tenantCtx, retentionPeriod); err != nil {
			slog.Warn("failed to delete old events past retention period", "error", err)
		}
	}

	return nil
}

func PushToSIEM() {
	_siemMtx.Lock()
	defer _siemMtx.Unlock()
	_pushToSiem = true
}

func SkipPushToSiem() {
	_siemMtx.Lock()
	defer _siemMtx.Unlock()
	_pushToSiem = false
}

func EventWatcher() {
	logic.HookManagerCh <- models.HookDetails{
		ID:       "events-retention-hook",
		Hook:     logic.WrapHook(EventRetentionHook),
		Interval: time.Hour * 24,
	}

	intgs, _ := (&schema.Integration{
		Type: string(integration.TypeSIEM),
	}).ListByType(db.WithContext(context.TODO()))
	if len(intgs) == 0 {
		SkipPushToSiem()
	} else if len(intgs) == 1 {
		PushToSIEM()
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
			TenantID:    e.TenantID,
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

		_siemMtx.Lock()
		if !_pushToSiem {
			_siemMtx.Unlock()
			continue
		}
		_siemMtx.Unlock()

		ctx := scope.WithContext(db.WithContext(context.TODO()), scope.TenantScope, e.TenantID)
		if logic.GetFeatureFlags(ctx).EnableSIEMIntegration {
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
				TenantId:    a.TenantID,
			})
		}
	}
}
