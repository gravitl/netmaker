package models

// Usage - struct for license usage
type Usage struct {
	Servers          int `json:"servers"`
	Users            int `json:"users"`
	Hosts            int `json:"hosts"`
	Clients          int `json:"clients"`
	Networks         int `json:"networks"`
	Ingresses        int `json:"ingresses"`
	Egresses         int `json:"egresses"`
	Relays           int `json:"relays"`
	InternetGateways int `json:"internet_gateways"`
}

// Usage.SetDefaults - sets the default values for usage
func (l *Usage) SetDefaults() {
	l.Clients = 0
	l.Servers = 1
	l.Hosts = 0
	l.Users = 1
	l.Networks = 0
	l.Ingresses = 0
	l.Egresses = 0
	l.Relays = 0
	l.InternetGateways = 0
}
