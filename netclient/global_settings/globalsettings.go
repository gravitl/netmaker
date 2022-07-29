package global_settings

// globalsettings - settings that are global in nature.  Avoids circular dependencies between config loading and usage.

// PublicIPServices - the list of user-specified IP services to use to obtain the node's public IP
var PublicIPServices map[string]string = make(map[string]string)
