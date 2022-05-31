package models

type AccessToken struct {
	APIConnString string `json:"apiconnstring"`
	ClientConfig
}

type ClientConfig struct {
	Network    string `json:"network"`
	Key        string `json:"key"`
	LocalRange string `json:"localrange"`
}
