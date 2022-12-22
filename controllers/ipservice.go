package controller

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/gravitl/netmaker/netclient/ncutils"
)

func ipHandlers(r *mux.Router) {
	r.HandleFunc("/api/getip", http.HandlerFunc(getPublicIP)).Methods("GET")
}

// swagger:route GET /api/getip ipservice getPublicIP
//
// Get the current public IP address.
//
//			Schemes: https
//
//			Security:
//	  		oauth
//
//			Responses:
//				200: byteArrayResponse
func getPublicIP(w http.ResponseWriter, r *http.Request) {
	r.Header.Set("Connection", "close")
	ip, err := parseIP(r)
	if err != nil {
		w.WriteHeader(400)
		switch {
		case ip != "":
			_, _ = w.Write([]byte("ip is invalid: " + ip))
		case ip == "":
			_, _ = w.Write([]byte("no ip found"))
		default:
			fmt.Println(err)
		}
		return
	}

	w.WriteHeader(200)
	_, _ = w.Write([]byte(ip))
}

func parseIP(r *http.Request) (string, error) {
	// Get Public IP from header
	ip := r.Header.Get("X-REAL-IP")
	ipnet := net.ParseIP(ip)
	if ipnet != nil && !ncutils.IpIsPrivate(ipnet) {
		return ip, nil
	}

	// If above fails, get Public IP from other header instead
	forwardips := r.Header.Get("X-FORWARDED-FOR")
	iplist := strings.Split(forwardips, ",")
	for _, ip := range iplist {
		ipnet := net.ParseIP(ip)
		if ipnet != nil && !ncutils.IpIsPrivate(ipnet) {
			return ip, nil
		}
	}

	// If above also fails, get Public IP from Remote Address of request
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return "", err
	}
	ipnet = net.ParseIP(ip)
	if ipnet != nil {
		if ncutils.IpIsPrivate(ipnet) {
			return ip, fmt.Errorf("ip is a private address")
		}
		return ip, nil
	}
	return "", fmt.Errorf("no ip found")
}
