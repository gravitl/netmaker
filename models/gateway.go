package models

type CreateGwReq struct {
	IngressRequest
	RelayRequest
	InetNodeReq
	TcpProxyEnabled        bool   `json:"tcp_proxy_enabled"`
	TcpProxyListenPort     int    `json:"tcp_proxy_listen_port"`
	TcpProxyTLSMode        string `json:"tcp_proxy_tls_mode"`
	TcpProxyListenAddr     string `json:"tcp_proxy_listen_addr,omitempty"`
	TcpProxyPublicHostname string `json:"tcp_proxy_public_hostname,omitempty"`
}

type DeleteGw struct {
}
