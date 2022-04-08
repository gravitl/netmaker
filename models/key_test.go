package models

import (
	"os"
	"testing"
)

func TestKey_Save(t *testing.T) {
	testKeyPath := "test.key"
	testKey, err := NewKey()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		key     *Key
		wantErr bool
	}{
		{
			"save-load",
			testKey,
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.key.Save(testKeyPath); (err != nil) != tt.wantErr {
				t.Errorf("Key.Save() error = %v, wantErr %v", err, tt.wantErr)
			}
			defer os.Remove(testKeyPath)
			if _, err := ReadFrom(testKeyPath); err != nil {
				t.Errorf("ReadFrom(%s) failed for newly saved key with err: %s", testKeyPath, err)
			}
		})
	}
}
