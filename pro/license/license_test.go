//go:build ee
// +build ee

package pro

import (
	"testing"

	"github.com/gravitl/netmaker/config"
	proLogic "github.com/gravitl/netmaker/pro/logic"
)

func Test_GetAccountsHost(t *testing.T) {
	tests := []struct {
		name string
		envK string
		envV string
		conf string
		want string
	}{
		{
			name: "no env var and no conf",
			envK: "NOT_THE_CORRECT_ENV_VAR",
			envV: "dev",
			want: "https://api.accounts.netmaker.io",
		},
		{
			name: "dev env var",
			envK: "ENVIRONMENT",
			envV: "dev",
			want: "https://api.dev.accounts.netmaker.io",
		},
		{
			name: "staging env var",
			envK: "ENVIRONMENT",
			envV: "staging",
			want: "https://api.staging.accounts.netmaker.io",
		},
		{
			name: "prod env var",
			envK: "ENVIRONMENT",
			envV: "prod",
			want: "https://api.accounts.netmaker.io",
		},
		{
			name: "dev conf",
			conf: "dev",
			want: "https://api.dev.accounts.netmaker.io",
		},
		{
			name: "staging conf",
			conf: "staging",
			want: "https://api.staging.accounts.netmaker.io",
		},
		{
			name: "prod conf",
			conf: "prod",
			want: "https://api.accounts.netmaker.io",
		},
		{
			name: "env var vs conf precedence",
			envK: "ENVIRONMENT",
			envV: "prod",
			conf: "staging",
			want: "https://api.accounts.netmaker.io",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Config.Server.Environment = tt.conf
			if tt.envK != "" {
				t.Setenv(tt.envK, tt.envV)
			}
			if got := proLogic.GetAccountsHost(); got != tt.want {
				t.Errorf("GetAccountsHost() = %v, want %v", got, tt.want)
			}
		})
	}
}
