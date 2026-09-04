package edr

import "testing"

func TestParseWazuhConfig_SkipTLSVerifyAlias(t *testing.T) {
	cfg, err := ParseWazuhConfig([]byte(`{
		"manager_url": "https://wazuh.example.com:55000",
		"username": "wazuh-wui",
		"password": "secret",
		"skip_tls_verify": true
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.InsecureSkipVerify {
		t.Fatal("expected skip_tls_verify to set InsecureSkipVerify")
	}
}
