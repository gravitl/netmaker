package stunserver

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/servercfg"
	"github.com/pkg/errors"
	"gortc.io/stun"
)

// Server is RFC 5389 basic server implementation.
//
// Current implementation is UDP only and not utilizes FINGERPRINT mechanism,
// nor ALTERNATE-SERVER, nor credentials mechanisms. It does not support
// backwards compatibility with RFC 3489.
type Server struct {
	Addr string
}

var (
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

func (s *Server) serveConn(c net.PacketConn, res, req *stun.Message, ctx context.Context) error {
	if c == nil {
		return nil
	}
	go func(ctx context.Context) {
		<-ctx.Done()
		if c != nil {
			// kill connection on server shutdown
			c.Close()
		}
	}(ctx)

	buf := make([]byte, 1024)
	n, addr, err := c.ReadFrom(buf) // this be blocky af
	if err != nil {
		if !strings.Contains(err.Error(), "use of closed network connection") {
			logger.Log(1, "STUN read error:", err.Error())
		}
		return nil
	}

	if _, err = req.Write(buf[:n]); err != nil {
		logger.Log(1, "STUN write error:", err.Error())
		return err
	}
	if err = basicProcess(addr, buf[:n], req, res); err != nil {
		if err == errNotSTUNMessage {
			return nil
		}
		logger.Log(1, "STUN process error:", err.Error())
		return nil
	}
	_, err = c.WriteTo(res.Raw, addr)
	if err != nil {
		logger.Log(1, "STUN response write error", err.Error())
	}
	return err
}

// Serve reads packets from connections and responds to BINDING requests.
func (s *Server) serve(c net.PacketConn, ctx context.Context) error {
	var (
		res = new(stun.Message)
		req = new(stun.Message)
	)
	for {
		select {
		case <-ctx.Done():
			logger.Log(0, "shut down STUN server")
			return nil
		default:
			if err := s.serveConn(c, res, req, ctx); err != nil {
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
	}
	return s.serve(c, ctx)
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
func Start(wg *sync.WaitGroup, ctx context.Context) {
	defer wg.Done()
	normalized := normalize(fmt.Sprintf("0.0.0.0:%d", servercfg.GetStunPort()))
	logger.Log(0, "netmaker-stun listening on", normalized, "via udp")
	if err := listenUDPAndServe(ctx, "udp", normalized); err != nil {
		if strings.Contains(err.Error(), "closed network connection") {
			logger.Log(0, "shutdown STUN server")
		} else {
			logger.Log(0, "server: ", err.Error())
		}
	}
}
