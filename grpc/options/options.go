package options

import (
	"crypto/tls"
	"time"

	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	DefaultBatchSize    = 200
	DefaultBatchTime    = 30 * time.Second
	DefaultRetryCount   = 3
	DefaultRetryBackoff = 300 * time.Millisecond
)

type Options struct {
	TLSCreds     credentials.TransportCredentials
	BatchSize    int
	BatchTime    time.Duration
	RetryCount   int
	RetryBackoff time.Duration
}

func Defaults() Options {
	return Options{
		TLSCreds:     insecure.NewCredentials(),
		BatchSize:    DefaultBatchSize,
		BatchTime:    DefaultBatchTime,
		RetryCount:   DefaultRetryCount,
		RetryBackoff: DefaultRetryBackoff,
	}
}

func WithTLS(cfg *tls.Config) func(*Options) {
	return func(o *Options) { o.TLSCreds = credentials.NewTLS(cfg) }
}

func WithBatchSize(n int) func(*Options) {
	return func(o *Options) { o.BatchSize = n }
}

func WithBatchTime(t time.Duration) func(*Options) {
	return func(o *Options) { o.BatchTime = t }
}

func WithRetryPolicy(count int, backoff time.Duration) func(*Options) {
	return func(o *Options) {
		o.RetryCount = count
		o.RetryBackoff = backoff
	}
}
