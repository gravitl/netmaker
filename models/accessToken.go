package models

// AccessToken - token used to access netmaker
type AccessToken struct {
	APIConnString string `json:"apiconnstring"`
	ClientConfig
}

// ClientConfig - the config of the client
type ClientConfig struct {
	Network    string `json:"network"`
	Key        string `json:"key"`
	LocalRange string `json:"localrange"`
}
