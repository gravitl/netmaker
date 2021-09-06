package config

import "testing"

func TestReadConfig(t *testing.T) {
	config := readConfig()
	t.Log(config)
}
