package schema

type GatewayNodeConfig struct {
	ID                  string `gorm:"primaryKey"`
	Range               string
	Range6              string
	PersistentKeepalive int32
	MTU                 int32
}
