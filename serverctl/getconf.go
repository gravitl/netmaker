package serverctl

func SetHost() error {
	remoteip, err := GetPublicIP()
	if err != nil {
		return err
	}
	os.Setenv("SERVER_HOST", remoteip)
}
func GetAPIHost() string {
        serverhost := 127.0.0.1
        if os.Getenv("SERVER_HTTP_HOST") != ""  {
                serverhost = os.Getenv("SERVER_HTTP_HOST")
        } else if config.Config.Server.APIHost != "" {
		serverhost = config.Config.Server.APIHost
        } else if os.Getenv("SERVER_HOST") != ""  {
                serverhost = os.Getenv("SERVER_HOST")
        }
	return serverhost
}
func GetAPIPort() string {
	apiport := "8081"
	if os.Getenv("API_PORT") != "" {
		apiport = os.Getenv("API_PORT")
	} else if  config.Config.Server.APIPort != "" {
		apiport = config.Config.Server.APIPort
	}
	return apiport
}
func GetGRPCHost() string {
        serverhost := 127.0.0.1
        if os.Getenv("SERVER_GRPC_HOST") != ""  {
                serverhost = os.Getenv("SERVER_GRPC_HOST")
        } else if config.Config.Server.GRPCHost != "" {
                serverhost = config.Config.Server.GRPCHost
        } else if os.Getenv("SERVER_HOST") != ""  {
                serverhost = os.Getenv("SERVER_HOST")
        }
        return serverhost
}
func GetGRPCPort() string {
        grpcport := "50051"
        if os.Getenv("GRPC_PORT") != "" {
                grpcport = os.Getenv("GRPC_PORT")
        } else if  config.Config.Server.GRPCPort != "" {
                grpcport = config.Config.Server.GRPCPort
        }
        return grpcport
}
func GetMasterKey() string {
        key := "secretkey"
        if os.Getenv("MASTER_KEY") != "" {
                key = os.Getenv("MASTER_KEY")
        } else if  config.Config.Server.MasterKey != "" {
                key = config.Config.Server.MasterKey
        }
        return key
}
func GetAllowedOrigin() string {
        allowedorigin := "*"
        if os.Getenv("CORS_ALLOWED_ORIGIN") != "" {
                allowedorigin = os.Getenv("CORS_ALLOWED_ORIGIN")
        } else if  config.Config.Server.AllowedOrigin != "" {
                allowedorigin = config.Config.Server.AllowedOrigin
        }
        return allowedorigin
}
func IsRestBackend() bool {
        isrest := true
        if os.Getenv("REST_BACKEND") != "" {
		if os.Getenv("REST_BACKEND") == "off"
			isrest = false
		}
	} else if config.Config.Server.RestBackend != "" {
		if config.Config.Server.RestBackend == "off" {
			isrest = false
		}
       }
       return isrest
}
func IsAgentBackend() bool {
        isagent := true
        if os.Getenv("AGENT_BACKEND") != "" {
                if os.Getenv("AGENT_BACKEND") == "off"
                        isagent = false
                }
        } else if config.Config.Server.AgentBackend != "" {
                if config.Config.Server.AgentBackend == "off" {
                        isagent = false
                }
       }
       return isagent
}
func IsClientMode() bool {
        isclient := true
        if os.Getenv("CLIENT_MODE") != "" {
                if os.Getenv("CLIENT_MODE") == "off"
                        isclient = false
                }
        } else if config.Config.Server.ClientMode != "" {
                if config.Config.Server.ClientMode == "off" {
                        isclient = false
                }
       }
       return isclient
}
func IsDNSMode() bool {
        isdns := true
        if os.Getenv("DNS_MODE") != "" {
                if os.Getenv("DNS_MODE") == "off"
                        isdns = false
                }
        } else if config.Config.Server.DNSMode != "" {
                if config.Config.Server.DNSMode == "off" {
                        isdns = false
                }
       }
       return isdns
}
func DisableRemoteIPCheck() bool {
        disabled := false
        if os.Getenv("DISABLE_REMOTE_IP_CHECK") != "" {
                if os.Getenv("DISABLE_REMOTE_IP_CHECK") == "on"
                        disabled = true
                }
        } else if config.Config.Server.DisableRemoteIpCheck != "" {
                if config.Config.Server.DisableRemoteIpCheck == "on" {
                        disabled= true
                }
       }
       return disabled
}
func GetPublicIP() (string, error) {

        endpoint := ""
        var err error

        iplist := []string{"http://ip.server.gravitl.com", "https://ifconfig.me", "http://api.ipify.org", "http://ipinfo.io/ip"}
        for _, ipserver := range iplist {
                resp, err := http.Get(ipserver)
                if err != nil {
                        continue
                }
                defer resp.Body.Close()
                if resp.StatusCode == http.StatusOK {
                        bodyBytes, err := ioutil.ReadAll(resp.Body)
                        if err != nil {
                                continue
                        }
                        endpoint = string(bodyBytes)
                        break
                }

        }
        if err == nil && endpoint == "" {
                err =  errors.New("Public Address Not Found.")
        }
        return endpoint, err
}
