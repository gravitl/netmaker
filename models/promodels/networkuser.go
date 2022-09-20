package promodels

// NetworkUserID - ID field for a network user
type NetworkUserID string

// NetworkUser - holds fields for a network user
type NetworkUser struct {
	AccessLevel int           `json:"accesslevel" bson:"accesslevel" yaml:"accesslevel"`
	ClientLimit int           `json:"clientlimit" bson:"clientlimit" yaml:"clientlimit"`
	NodeLimit   int           `json:"nodelimit" bson:"nodelimit" yaml:"nodelimit"`
	ID          NetworkUserID `json:"id" bson:"id" yaml:"id"`
	Clients     []string      `json:"clients" bson:"clients" yaml:"clients"`
	Nodes       []string      `json:"nodes" bson:"nodes" yaml:"nodes"`
}

// NetworkUserMap - map of network users
type NetworkUserMap map[NetworkUserID]NetworkUser

// NetworkUserMap.Delete - deletes a network user struct from a given map in memory
func (N NetworkUserMap) Delete(ID NetworkUserID) {
	delete(N, ID)
}

// NetworkUserMap.Add - adds a network user struct to given network user map in memory
func (N NetworkUserMap) Add(User *NetworkUser) {
	N[User.ID] = *User
}

// SetDefaults - adds the defaults to network user
func (U *NetworkUser) SetDefaults() {
	if U.Clients == nil {
		U.Clients = []string{}
	}
	if U.Nodes == nil {
		U.Nodes = []string{}
	}
}
