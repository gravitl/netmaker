//Environment file for getting variables
//Currently the only thing it does is set the master password
//Should probably have it take over functions from OS such as port and mongodb connection details
//Reads from the config/environments/dev.yaml file by default
package config

import (
	"reflect"
	"testing"
)

func Test_readConfig(t *testing.T) {
	tests := []struct {
		name    string
		want    *EnvironmentConfig
		wantErr bool
	}{
		{
			"ensure development config parses",
			&EnvironmentConfig{},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ReadConfig("")
			if (err != nil) != tt.wantErr {
				t.Errorf("readConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("readConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}
