package stunserver

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"gortc.io/stun"
)

// Server is RFC 5389 basic server implementation.
//
// Current implementation is UDP only and not utilizes FINGERPRINT mechanism,
// nor ALTERNATE-SERVER, nor credentials mechanisms. It does not support
// backwards compatibility with RFC 3489.
type Server struct {
	Addr string
	Ctx  context.Context
}

// Logger is used for logging formatted messages.
type Logger interface {
	// Printf must have the same semantics as log.Printf.
	Printf(format string, args ...interface{})
}

var (
	defaultLogger     = logrus.New()
	software          = stun.NewSoftware("netmaker-stun")
	errNotSTUNMessage = errors.New("not stun message")
)

func basicProcess(addr net.Addr, b []byte, req, res *stun.Message) error {
	if !stun.IsMessage(b) {
		return errNotSTUNMessage
	}
	if _, err := req.Write(b); err != nil {
		return errors.Wrap(err, "failed to read message")
	}
	var (
		ip   net.IP
		port int
	)
	switch a := addr.(type) {
	case *net.UDPAddr:
		ip = a.IP
		port = a.Port
	default:
		panic(fmt.Sprintf("unknown addr: %v", addr))
	}
	return res.Build(req,
		stun.BindingSuccess,
		software,
		&stun.XORMappedAddress{
			IP:   ip,
			Port: port,
		},
		stun.Fingerprint,
	)
}

func (s *Server) serveConn(c net.PacketConn, res, req *stun.Message) error {
	if c == nil {
		return nil
	}
	buf := make([]byte, 1024)
	n, addr, err := c.ReadFrom(buf)
	if err != nil {
		logger.Log(1, "ReadFrom: %v", err.Error())
		return nil
	}
	log.Printf("read %d bytes from %s\n", n, addr)
	if _, err = req.Write(buf[:n]); err != nil {
		logger.Log(1, "Write: %v", err.Error())
		return err
	}
	if err = basicProcess(addr, buf[:n], req, res); err != nil {
		if err == errNotSTUNMessage {
			return nil
		}
		logger.Log(1, "basicProcess: %v", err.Error())
		return nil
	}
	_, err = c.WriteTo(res.Raw, addr)
	if err != nil {
		logger.Log(1, "WriteTo: %v", err.Error())
	}
	return err
}

// Serve reads packets from connections and responds to BINDING requests.
func (s *Server) serve(c net.PacketConn) error {
	var (
		res = new(stun.Message)
		req = new(stun.Message)
	)
	for {
		select {
		case <-s.Ctx.Done():
			logger.Log(0, "Shutting down stun server...")
			c.Close()
			return nil
		default:
			if err := s.serveConn(c, res, req); err != nil {
				logger.Log(1, "serve: %v", err.Error())
				continue
			}
			res.Reset()
			req.Reset()
		}
	}
}

// listenUDPAndServe listens on laddr and process incoming packets.
func listenUDPAndServe(ctx context.Context, serverNet, laddr string) error {
	c, err := net.ListenPacket(serverNet, laddr)
	if err != nil {
		return err
	}
	s := &Server{
		Addr: laddr,
		Ctx:  ctx,
	}
	return s.serve(c)
}

func normalize(address string) string {
	if len(address) == 0 {
		address = "0.0.0.0"
	}
	if !strings.Contains(address, ":") {
		address = fmt.Sprintf("%s:%d", address, stun.DefaultPort)
	}
	return address
}

// Start - starts the stun server
func Start(wg *sync.WaitGroup) {
	ctx, cancel := context.WithCancel(context.Background())
	go func(wg *sync.WaitGroup) {
		defer wg.Done()
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGTERM, os.Interrupt)
		<-quit
		cancel()
	}(wg)
	normalized := normalize(fmt.Sprintf("0.0.0.0:%d", servercfg.GetStunPort()))
	logger.Log(0, "netmaker-stun listening on", normalized, "via udp")
	err := listenUDPAndServe(ctx, "udp", normalized)
	if err != nil {
		logger.Log(0, "failed to start stun server: ", err.Error())
	}
}
