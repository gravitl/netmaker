package siem

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"

	"github.com/gravitl/netmaker/grpc/options"
	"github.com/gravitl/netmaker/servercfg"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	defaultClient     *GrpcClient
	defaultClientOnce sync.Once
)

type GrpcClient struct {
	serverAddr string
	opts       options.Options
}

func NewSIEMGrpcClient(serverAddr string, optFns ...func(*options.Options)) *GrpcClient {
	opts := options.Defaults()
	for _, fn := range optFns {
		fn(&opts)
	}

	return &GrpcClient{
		serverAddr: serverAddr,
		opts:       opts,
	}
}

func Client() *GrpcClient {
	defaultClientOnce.Do(func() {
		defaultClient = NewSIEMGrpcClient(
			fmt.Sprintf("grpc.%s", servercfg.GetNmBaseDomain()),
			options.WithTLS(&tls.Config{}),
		)
	})
	return defaultClient
}

func (c *GrpcClient) dial() (*grpc.ClientConn, SIEMServiceClient, error) {
	conn, err := grpc.NewClient(c.serverAddr, grpc.WithTransportCredentials(c.opts.TLSCreds))
	if err != nil {
		return nil, nil, fmt.Errorf("dial: %w", err)
	}
	return conn, NewSIEMServiceClient(conn), nil
}

func (c *GrpcClient) Init(ctx context.Context, providerID string, config *structpb.Struct) error {
	conn, client, err := c.dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	resp, err := client.InitSIEM(ctx, &InitSIEMRequest{
		ProviderId: providerID,
		Config:     config,
	})
	if err != nil {
		return fmt.Errorf("InitSIEM: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("InitSIEM rejected: %s", resp.Error)
	}

	return nil
}

func (c *GrpcClient) Terminate(ctx context.Context) error {
	conn, client, err := c.dial()
	if err != nil {
		return err
	}
	defer conn.Close()

	resp, err := client.TerminateSIEM(ctx, &emptypb.Empty{})
	if err != nil {
		return fmt.Errorf("TerminateSIEM: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("TerminateSIEM rejected: %s", resp.Error)
	}

	return nil
}
