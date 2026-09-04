package db

import (
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm/logger"
)

// newGormLogger returns a GORM logger.
//
// Set SQL_DEBUG=true (or "1" / "info") to log every SQL statement with duration.
// Set SQL_DEBUG=slow to only log statements slower than SQL_SLOW_MS (default 200ms)
// plus errors. Default remains Silent.
func newGormLogger() logger.Interface {
	mode := strings.ToLower(strings.TrimSpace(os.Getenv("SQL_DEBUG")))
	if mode == "" || mode == "false" || mode == "0" {
		return logger.Default.LogMode(logger.Silent)
	}

	slowMs := 200 * time.Millisecond
	if v := os.Getenv("SQL_SLOW_MS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			slowMs = time.Duration(n) * time.Millisecond
		}
	}

	level := logger.Info
	if mode == "slow" || mode == "warn" {
		level = logger.Warn
	}

	return logger.New(
		log.New(os.Stdout, "[gorm] ", log.LstdFlags|log.Lmicroseconds),
		logger.Config{
			SlowThreshold:             slowMs,
			LogLevel:                  level,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)
}
