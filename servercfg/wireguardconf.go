package servercfg

import (
        "github.com/gravitl/netmaker/config"
        "os"
)
func IsRegisterKeyRequired() bool {
       isrequired := false
       if os.Getenv("SERVER_GRPC_WG_KEYREQUIRED") != "" {
                if os.Getenv("SERVER_GRPC_WG_KEYREQUIRED") == "yes" {
                        isrequired = true
                }
       } else if config.Config.WG.RegisterKeyRequired != "" {
                if config.Config.WG.RegisterKeyRequired == "yes" {
                        isrequired = true
                }
       }
       return isrequired
}
func IsGRPCWireGuard() bool {
       iswg := true
       if os.Getenv("SERVER_GRPC_WIREGUARD") != "" {
                if os.Getenv("SERVER_GRPC_WIREGUARD") == "off" {
                        iswg = false
                }
       } else if config.Config.WG.GRPCWireGuard != "" {
                if config.Config.WG.GRPCWireGuard == "off" {
                        iswg = false
                }
       }
       return iswg
}
func GetGRPCWGInterface() string {
       iface := "nm-grpc-wg"
       if os.Getenv("SERVER_GRPC_WG_INTERFACE") != "" {
                iface = os.Getenv("SERVER_GRPC_WG_INTERFACE")
       } else if config.Config.WG.GRPCWGInterface != "" {
                iface = config.Config.WG.GRPCWGInterface
       }
       return iface
}
func GetGRPCWGAddress() string {
        address := "10.101.0.1"
      if os.Getenv("SERVER_GRPC_WG_ADDRESS") != ""  {
              address = os.Getenv("SERVER_GRPC_WG_ADDRESS")
      } else if config.Config.WG.GRPCWGAddress != "" {
              address = config.Config.WG.GRPCWGAddress
      }
      return address
}
func GetGRPCWGAddressRange() string {
        address := "fd73:0093:84f3:a13d::/64"
      if os.Getenv("SERVER_GRPC_WG_ADDRESS_RANGE") != ""  {
              address = os.Getenv("SERVER_GRPC_WG_ADDRESS_RANGE")
      } else if config.Config.WG.GRPCWGAddressRange != "" {
              address = config.Config.WG.GRPCWGAddressRange
      }
      return address
}
func GetGRPCWGPort() string {
        port := "50555"
      if os.Getenv("SERVER_GRPC_WG_PORT") != ""  {
              port = os.Getenv("SERVER_GRPC_WG_PORT")
      } else if config.Config.WG.GRPCWGPort != "" {
              port = config.Config.WG.GRPCWGPort
      }
      return port
}

func GetGRPCWGPubKey() string {
      key := os.Getenv("SERVER_GRPC_WG_PUBKEY")
      if config.Config.WG.GRPCWGPubKey != "" {
              key = config.Config.WG.GRPCWGPubKey
      }
      return key
}

func GetGRPCWGPrivKey() string {
      key := os.Getenv("SERVER_GRPC_WG_PRIVKEY")
      if config.Config.WG.GRPCWGPrivKey != "" {
              key = config.Config.WG.GRPCWGPrivKey
      }
      return key
}

