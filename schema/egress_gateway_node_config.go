package schema

type EgressGatewayNodeConfig struct {
	ID         string `gorm:"primaryKey"`
	NatEnabled bool
	Ranges     []RangeWithMetric `gorm:"serializer:json"`
}

type RangeWithMetric struct {
	Range  string `json:"range"`
	Metric uint32 `json:"metric"`
}
