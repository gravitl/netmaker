package servercfg

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gravitl/netmaker/config"
)

func TestGetPublicIP(t *testing.T) {
	var testIP = "55.55.55.55"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte(testIP)); err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	}))
	defer server.Close()
	if err := os.Setenv("NETMAKER_TEST_IP_SERVICE", server.URL); err != nil {
		t.Logf("WARNING: could not set NETMAKER_TEST_IP_SERVICE env var")
	}

	// 1. Test that the function checks the environment variable PUBLIC_IP_SERVICE first.
	t.Run("Use PUBLIC_IP_SERVICE if set", func(t *testing.T) {

		// set the environment variable
		if err := os.Setenv("PUBLIC_IP_SERVICE", server.URL); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		defer func() {
			_ = os.Unsetenv("PUBLIC_IP_SERVICE")
		}()

		var ip string
		var err error
		if ip, err = GetPublicIP(); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if !strings.EqualFold(ip, testIP) {
			t.Errorf("expected IP to be %s, got %s", testIP, ip)
		}
	})

	t.Run("Use config.Config.Server.PublicIPService if PUBLIC_IP_SERVICE isn't set", func(t *testing.T) {
		// Mock the config
		config.Config.Server.PublicIPService = server.URL
		defer func() { config.Config.Server.PublicIPService = "" }()

		ip, err := GetPublicIP()
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
		if ip != testIP {
			t.Fatalf("expected IP to be %s, got %s", testIP, ip)
		}
	})

	t.Run("Handle service timeout", func(t *testing.T) {
		if os.Getenv("NETMAKER_TEST_IP_SERVICE") == "" {
			t.Skip("NETMAKER_TEST_IP_SERVICE not set")
		}

		var badTestIP = "123.45.67.91"
		badServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// intentionally delay to simulate a timeout
			time.Sleep(3 * time.Second)
			_, _ = w.Write([]byte(badTestIP))
		}))
		defer badServer.Close()

		// we set this so that we can lower the timeout for the test to not hold up CI
		if err := os.Setenv("NETMAKER_TEST_BAD_IP_SERVICE", badServer.URL); err != nil {
			// but if we can't set it, we can't test it
			t.Skip("failed to set NETMAKER_TEST_BAD_IP_SERVICE, skipping test because we won't timeout")
		}
		defer func() {
			_ = os.Unsetenv("NETMAKER_TEST_BAD_IP_SERVICE")
		}()

		// mock the config
		oldConfig := config.Config.Server.PublicIPService
		config.Config.Server.PublicIPService = badServer.URL
		defer func() { config.Config.Server.PublicIPService = oldConfig }()

		res, err := GetPublicIP()
		if err != nil {
			t.Errorf("GetPublicIP() fallback has failed: %v", err)
		}
		if strings.EqualFold(res, badTestIP) {
			t.Errorf("GetPublicIP() returned the response from the server that should have timed out: %v", res)
		}
		if !strings.EqualFold(res, testIP) {
			t.Errorf("GetPublicIP() did not fallback to the correct IP: %v", res)
		}
	})

}

// Note: The rest of the GetPublicIP function would remain unchanged in this file.
