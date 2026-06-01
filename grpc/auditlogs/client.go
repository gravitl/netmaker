package auditlogs

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gravitl/netmaker/grpc/options"
	"github.com/gravitl/netmaker/servercfg"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const DefaultAuditBatchTime = 5 * time.Second

var (
	defaultClient     *GrpcClient
	defaultClientOnce sync.Once
)

type GrpcClient struct {
	serverAddr string
	opts       options.Options

	conn   *grpc.ClientConn
	stream AuditLogService_StreamAuditLogsClient

	mu     sync.Mutex
	events []*AuditLogEvent

	batchLoopOnce sync.Once
	stopCh        chan struct{}
	wg            sync.WaitGroup
}

func NewAuditLogsGrpcClient(serverAddr string, optFns ...func(*options.Options)) *GrpcClient {
	opts := options.Defaults()
	opts.BatchTime = DefaultAuditBatchTime
	for _, fn := range optFns {
		fn(&opts)
	}

	return &GrpcClient{
		serverAddr: serverAddr,
		opts:       opts,
		stopCh:     make(chan struct{}),
	}
}

func Client() *GrpcClient {
	defaultClientOnce.Do(func() {
		defaultClient = NewAuditLogsGrpcClient(
			fmt.Sprintf("grpc.%s", servercfg.GetNmBaseDomain()),
			options.WithTLS(&tls.Config{}),
		)

		// The default client is lazy. It connects only when an export is requested.
		// But it still needs to start a batch loop to flush the events cache.
		defaultClient.ensureBatchLoop()
	})
	return defaultClient
}

func (c *GrpcClient) Start() error {
	err := c.connect()
	if err != nil {
		return err
	}

	defaultClient.ensureBatchLoop()
	return nil
}

func (c *GrpcClient) Stop() error {
	close(c.stopCh)
	c.wg.Wait()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

func (c *GrpcClient) Export(event *AuditLogEvent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.events = append(c.events, event)
	if len(c.events) >= c.opts.BatchSize {
		go c.flush()
	}
	return nil
}

func (c *GrpcClient) ensureBatchLoop() {
	c.batchLoopOnce.Do(func() {
		c.wg.Add(1)
		go c.batchLoop()
	})
}

func (c *GrpcClient) batchLoop() {
	defer c.wg.Done()

	ticker := time.NewTicker(c.opts.BatchTime)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			c.flush()
		case <-c.stopCh:
			c.flush()
			return
		}
	}
}

func (c *GrpcClient) flush() {
	fmt.Println("FLUSHING AUDIT LOGS")
	c.mu.Lock()
	if len(c.events) == 0 {
		c.mu.Unlock()
		return
	}
	evs := c.events
	c.events = nil
	c.mu.Unlock()

	env := &AuditLogEnvelope{Events: evs}

	if err := c.sendWithRetries(env); err != nil {
		fmt.Println("[auditlogs] permanently dropped batch:", err)
	}
}

func (c *GrpcClient) sendWithRetries(env *AuditLogEnvelope) error {
	var err error

	for attempt := 1; attempt <= c.opts.RetryCount; attempt++ {
		fmt.Println("FLUSHING AUDIT LOGS ATTEMPT:", attempt)
		err = c.sendOnce(env)
		if err == nil {
			return nil
		}

		fmt.Printf("[auditlogs] send attempt %d failed: %v\n", attempt, err)
		time.Sleep(c.opts.RetryBackoff)
	}

	return fmt.Errorf("retry limit exceeded: %w", err)
}

func (c *GrpcClient) sendOnce(env *AuditLogEnvelope) error {
	if c.stream == nil {
		fmt.Println("FLUSHING AUDIT LOGS CONNECTING")
		if err := c.reconnect(); err != nil {
			return err
		}
	}

	fmt.Println("FLUSHING AUDIT LOGS CONNECTED")
	if err := c.stream.Send(env); err != nil {
		return c.handleStreamError(err)
	}

	resp, err := c.stream.Recv()
	if err != nil {
		return c.handleStreamError(err)
	}

	if !resp.Success {
		return fmt.Errorf("server rejected: %s", resp.Error)
	}

	return nil
}

func (c *GrpcClient) connect() error {
	conn, err := grpc.NewClient(c.serverAddr, grpc.WithTransportCredentials(c.opts.TLSCreds))
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}

	c.conn = conn

	stream, err := NewAuditLogServiceClient(conn).StreamAuditLogs(context.Background())
	if err != nil {
		return fmt.Errorf("open stream: %w", err)
	}

	c.stream = stream
	return nil
}

func (c *GrpcClient) reconnect() error {
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.stream = nil
	time.Sleep(c.opts.RetryBackoff)
	return c.connect()
}

func (c *GrpcClient) handleStreamError(err error) error {
	if err == io.EOF {
		if recErr := c.reconnect(); recErr != nil {
			return recErr
		}
		return fmt.Errorf("stream closed: %w", err)
	}

	if st, ok := status.FromError(err); ok {
		return fmt.Errorf("grpc status: %s", st.Message())
	}

	return err
}
