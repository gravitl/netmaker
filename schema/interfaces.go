package schema

type Interface struct {
	HostID  string `gorm:"primaryKey"`
	Name    string `gorm:"primaryKey"`
	Address string
}
