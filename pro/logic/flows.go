package logic

import (
	"context"
	"errors"
	"fmt"
	"time"

	ch "github.com/gravitl/netmaker/clickhouse"
	"github.com/gravitl/netmaker/logic"
	"github.com/gravitl/netmaker/models"
)

const (
	flowsCleanupHookID       = "flows-cleanup-hook"
	flowsCleanupHookInterval = 24 * time.Hour
)

func StartFlowCleanupLoop() {
	logic.HookManagerCh <- models.HookDetails{
		ID:       flowsCleanupHookID,
		Hook:     logic.WrapHook(CleanupFlows),
		Interval: flowsCleanupHookInterval,
	}
}

func StopFlowCleanupLoop() {
	logic.StopHook(flowsCleanupHookID)
}

func CleanupFlows() error {
	ctx := ch.WithContext(context.TODO())
	conn, err := ch.FromContext(ctx)
	if err != nil {
		return fmt.Errorf("clickhouse connection not available: %w", err)
	}
	rows, err := conn.Query(ctx, `
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
			err = conn.Exec(ctx, fmt.Sprintf("ALTER TABLE netmaker.flows DROP partition %s", partitionID))
			if err != nil {
				cleanErr = errors.Join(cleanErr, err)
				continue
			}
		}
	}
	return cleanErr
}
