package models

type AccessToken struct {
	ServerConfig
	ClientConfig
}

type ClientConfig struct {
	Network    string `json:"network"`
	Key        string `json:"key"`
	LocalRange string `json:"localrange"`
}

type ServerConfig struct {
	CoreDNSAddr     string `json:"corednsaddr"`
	GRPCConnString  string `json:"grpcconn"`
	GRPCSSL         string `json:"grpcssl"`
	CheckinInterval string `json:"checkininterval"`
}
