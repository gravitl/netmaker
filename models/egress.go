package models

type EgressReq struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Network     string         `json:"network"`
	Description string         `json:"description"`
	Nodes       map[string]int `json:"nodes"`
	Tags        []string       `json:"tags"`
	Range       string         `json:"range"`
	Nat         bool           `json:"nat"`
	IsInetGw    bool           `json:"is_internet_gateway"`
}
