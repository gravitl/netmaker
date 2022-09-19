package promodels

// ProNetwork - struct for all pro Network related fields
type ProNetwork struct {
	DefaultAccessLevel     int      `json:"defaultaccesslevel" bson:"defaultaccesslevel" yaml:"defaultaccesslevel"`
	DefaultUserNodeLimit   int      `json:"defaultusernodelimit" bson:"defaultusernodelimit" yaml:"defaultusernodelimit"`
	DefaultUserClientLimit int      `json:"defaultuserclientlimit" bson:"defaultuserclientlimit" yaml:"defaultuserclientlimit"`
	AllowedUsers           []string `json:"allowedusers" bson:"allowedusers" yaml:"allowedusers"`
	AllowedGroups          []string `json:"allowedgroups" bson:"allowedgroups" yaml:"allowedgroups"`
}

// LoginMsg - login message struct for nodes to join via SSO login
// Need to change mac to public key for tighter verification ?
type LoginMsg struct {
	Mac      string `json:"mac"`
	Network  string `json:"network"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
}
