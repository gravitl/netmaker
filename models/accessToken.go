package models

type AccessToken struct {
	ServerConfig
	ClientConfig
	WG
}

type ClientConfig struct {
  Network string `json:"network"`
  Key string `json:"key"`
  LocalRange string `json:"localrange"`
}

type ServerConfig struct {
  CoreDNSAddr string `json:"corednsaddr"`
  APIConnString string `json:"apiconn"`
  APIHost   string  `json:"apihost"`
  APIPort   string `json:"apiport"`
  GRPCConnString string `json:"grpcconn"`
  GRPCHost   string `json:"grpchost"`
  GRPCPort   string `json:"grpcport"`
  GRPCSSL   string `json:"grpcssl"`
  CheckinInterval   string `json:"checkininterval"`
}

type WG struct {
  GRPCWireGuard  string  `json:"grpcwg"`
  GRPCWGAddress  string `json:"grpcwgaddr"`
  GRPCWGPort  string  `json:"grpcwgport"`
  GRPCWGPubKey  string  `json:"grpcwgpubkey"`
  GRPCWGEndpoint  string  `json:"grpcwgendpoint"`
}
