package logic

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	ch "github.com/gravitl/netmaker/clickhouse"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

type FlowsCleanupManager struct {
	cancel context.CancelFunc
	mu     sync.Mutex
}

var manager = &FlowsCleanupManager{}

func GetFlowsCleanupManager() *FlowsCleanupManager {
	return manager
}

func (f *FlowsCleanupManager) Start() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cancel()

	ctx, cancel := context.WithCancel(context.Background())
	f.cancel = cancel

	logic.HookManagerCh <- models.HookDetails{
		Ctx:      ctx,
		Hook:     CleanupFlows,
		Interval: 24 * time.Hour,
	}
}

func (f *FlowsCleanupManager) Stop() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.cancel()
}

func StartFlowCleanupLoop() {
	GetFlowsCleanupManager().Start()
}

func StopFlowCleanupLoop() {
	GetFlowsCleanupManager().Stop()
}

func CleanupFlows() error {
	ctx := ch.WithContext(context.TODO())
	rows, err := ch.FromContext(ctx).Query(ctx, `
SELECT DISTINCT parts.partition
FROM system.parts
WHERE parts.database = 'netmaker' AND parts.table = 'flows'
ORDER BY parts.partition ASC
`)
	if err != nil {
		return err
	}
	defer rows.Close()

	cutoff := time.Now().AddDate(0, 0, -1*logic.GetServerSettings().AuditLogsRetentionPeriodInDays)

	var cleanErr error
	for rows.Next() {
		var partitionID string
		err = rows.Scan(&partitionID)
		if err != nil {
			cleanErr = errors.Join(cleanErr, err)
			continue
		}

		partition, err := time.Parse("20060102", partitionID)
		if err != nil {
			cleanErr = errors.Join(cleanErr, err)
			continue
		}

		if partition.Before(cutoff) {
			err = ch.FromContext(ctx).Exec(ctx, fmt.Sprintf("ALTER TABLE netmaker.flows DROP partition %s", partitionID))
			if err != nil {
				cleanErr = errors.Join(cleanErr, err)
				continue
			}
		}
	}
	return cleanErr
}
