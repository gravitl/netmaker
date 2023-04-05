package models

type HostRegister struct {
	HostID       string `json:"host_id"`
	HostPassHash string `json:"host_pass_hash"`
}
