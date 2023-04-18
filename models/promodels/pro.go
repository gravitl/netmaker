package promodels

// ProNetwork - struct for all pro Network related fields
type ProNetwork struct {
	DefaultAccessLevel     int      `json:"defaultaccesslevel" bson:"defaultaccesslevel" yaml:"defaultaccesslevel"`
	DefaultUserNodeLimit   int      `json:"defaultusernodelimit" bson:"defaultusernodelimit" yaml:"defaultusernodelimit"`
	DefaultUserClientLimit int      `json:"defaultuserclientlimit" bson:"defaultuserclientlimit" yaml:"defaultuserclientlimit"`
	AllowedUsers           []string `json:"allowedusers" bson:"allowedusers" yaml:"allowedusers"`
	AllowedGroups          []string `json:"allowedgroups" bson:"allowedgroups" yaml:"allowedgroups"`
}
