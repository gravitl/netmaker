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
	Server        string `json:"server"`
	APIConnString string `json:"apiconnstring"`
}
