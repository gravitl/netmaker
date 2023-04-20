package turn

import (
	"context"
	"encoding/base64"
	"log"
	"net"
	"strconv"
	"sync"
	"syscall"

	"github.com/gravitl/netmaker/logger"
	"github.com/gravitl/netmaker/turnserver/config"
	"github.com/gravitl/netmaker/turnserver/internal/auth"
	"github.com/gravitl/netmaker/turnserver/internal/utils"
	"github.com/pion/turn/v2"
	"golang.org/x/sys/unix"
)

func Start(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	// Create a UDP listener to pass into pion/turn
	// pion/turn itself doesn't allocate any UDP sockets, but lets the user pass them in
	// this allows us to add logging, storage or modify inbound/outbound traffic
	// udpListener, err := net.ListenPacket("udp4", "0.0.0.0:"+strconv.Itoa(config.GetTurnPort()))
	// if err != nil {
	// 	log.Panicf("Failed to create TURN server listener: %s", err)
	// }
	addr, err := net.ResolveUDPAddr("udp", "0.0.0.0:"+strconv.Itoa(config.GetTurnPort()))
	if err != nil {
		log.Fatalf("Failed to parse server address: %s", err)
	}
	publicIP, err := utils.GetPublicIP()
	if err != nil {
		logger.FatalLog("failed to get public ip: ", err.Error())
	}
	listenerConfig := &net.ListenConfig{
		Control: func(network, address string, conn syscall.RawConn) error {
			var operr error
			if err = conn.Control(func(fd uintptr) {
				operr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, unix.SO_REUSEPORT, 1)
			}); err != nil {
				return err
			}

			return operr
		},
	}

	relayAddressGenerator := &turn.RelayAddressGeneratorStatic{
		RelayAddress: net.ParseIP(publicIP), // Claim that we are listening on IP passed by user
		Address:      "0.0.0.0",             // But actually be listening on every interface
	}

	packetConnConfigs := make([]turn.PacketConnConfig, 1)
	for i := 0; i < 1; i++ {
		conn, listErr := listenerConfig.ListenPacket(context.Background(), addr.Network(), addr.String())
		if listErr != nil {
			log.Fatalf("Failed to allocate UDP listener at %s:%s", addr.Network(), addr.String())
		}

		packetConnConfigs[i] = turn.PacketConnConfig{
			PacketConn:            conn,
			RelayAddressGenerator: relayAddressGenerator,
		}

		log.Printf("Server %d listening on %s\n", i, conn.LocalAddr().String())
	}

	s, err := turn.NewServer(turn.ServerConfig{
		Realm: config.GetTurnHost(),
		// Set AuthHandler callback
		// This is called every time a user tries to authenticate with the TURN server
		// Return the key for that user, or false when no user is found
		AuthHandler: func(username string, realm string, srcAddr net.Addr) ([]byte, bool) {
			if key, ok := auth.HostMap[username]; ok {
				keyBytes, err := base64.StdEncoding.DecodeString(key)
				if err != nil {
					return nil, false
				}
				return keyBytes, true
			}
			return nil, false
		},
		//ChannelBindTimeout: time.Duration(time.Minute),
		// PacketConnConfigs is a list of UDP Listeners and the configuration around them
		PacketConnConfigs: packetConnConfigs,
	})
	if err != nil {
		log.Panic(err)
	}
	// Block until user sends SIGINT or SIGTERM
	<-ctx.Done()
	logger.Log(0, "## Stopping Turn Server...")
	if err = s.Close(); err != nil {
		log.Panic(err)
	}
}
