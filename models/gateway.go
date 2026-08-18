package models

type CreateGwReq struct {
	IngressRequest
	RelayRequest
	InetNodeReq
	TcpProxyEnabled    bool `json:"tcp_proxy_enabled"`
	TcpProxyListenPort int  `json:"tcp_proxy_listen_port"`
}

// TcpProxyReq updates TCP proxy listen settings (persisted on host; node kept in sync for API).
type TcpProxyReq struct {
	Enabled    bool `json:"enabled"`
	ListenPort int  `json:"listen_port"`
}

type DeleteGw struct {
}
