package schema

import "gorm.io/datatypes"

type EgressGatewayNodeConfig struct {
	ID         string `gorm:"primaryKey"`
	NatEnabled bool
	Ranges     datatypes.JSONSlice[RangeWithMetric]
}

type RangeWithMetric struct {
	Range  string `json:"range"`
	Metric uint32 `json:"metric"`
}
