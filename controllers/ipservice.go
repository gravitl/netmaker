package controller

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

func ipHandlers(r *mux.Router) {
	r.HandleFunc("/api/getip", http.HandlerFunc(getPublicIP)).Methods("GET")
}

func getPublicIP(w http.ResponseWriter, r *http.Request) {
	r.Header.Set("Connection", "close")
	ip, err := parseIP(r)
	if err != nil {
		w.WriteHeader(400)
		if ip != "" {
			w.Write([]byte("ip is invalid: " + ip))
			return
		} else {
			w.Write([]byte("no ip found"))
			return
		}
	} else {
		if err != nil {
			fmt.Println(err)
		}
	}
	w.WriteHeader(200)
	w.Write([]byte(ip))
}

func parseIP(r *http.Request) (string, error) {
	// Get Public IP from header
	ip := r.Header.Get("X-REAL-IP")
	ipnet := net.ParseIP(ip)
	if ipnet != nil && !ipIsPrivate(ipnet) {
		return ip, nil
	}

	// If above fails, get Public IP from other header instead
	forwardips := r.Header.Get("X-FORWARDED-FOR")
	iplist := strings.Split(forwardips, ",")
	for _, ip := range iplist {
		ipnet := net.ParseIP(ip)
		if ipnet != nil && !ipIsPrivate(ipnet) {
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
		if ipIsPrivate(ipnet) {
			return ip, fmt.Errorf("ip is a private address")
		}
		return ip, nil
	}
	return "", fmt.Errorf("no ip found")
}

func ipIsPrivate(ipnet net.IP) bool {
	return ipnet.IsPrivate() || ipnet.IsLoopback()
}
