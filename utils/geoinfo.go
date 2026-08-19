package utils

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

type GeoInfo struct {
	IP          string
	CountryCode string
	Location    string
}

// GetGeoInfo returns the ip, location and country code of the host it's called on.
func GetGeoInfo(ip ...net.IP) (*GeoInfo, error) {
	geoInfo, err := getGeoInfoFromIPAPI(ip...)
	if err == nil && geoInfo.HasUsableGeo() {
		return geoInfo, nil
	}

	return getGeoInfoFromIpInfo(ip...)
}

// IsEmpty reports whether the provider returned no useful geo fields.
func (g *GeoInfo) IsEmpty() bool {
	return g.IP == "" && g.CountryCode == "" && g.Location == ""
}

// HasUsableGeo reports whether payload contains meaningful geo data.
func (g *GeoInfo) HasUsableGeo() bool {
	if g == nil || g.IP == "" {
		return false
	}
	if g.CountryCode != "" {
		return true
	}
	location := strings.TrimSpace(g.Location)
	if location == "" || location == "0,0" || location == "0.000000,0.000000" {
		return false
	}
	return true
}

func getGeoInfoFromIPAPI(ip ...net.IP) (*GeoInfo, error) {
	hosts := []string{"api.ipapi.is", "us.ipapi.is", "de.ipapi.is", "sg.ipapi.is"}
	client := http.Client{Timeout: 5 * time.Second}

	var lastErr error
	for _, host := range hosts {
		url := fmt.Sprintf("https://%s", host)
		if len(ip) > 0 {
			url = fmt.Sprintf("https://%s/?q=%s", host, ip[0].String())
		}

		resp, err := client.Get(url)
		if err != nil {
			lastErr = err
			continue
		}

		var geo *GeoInfo
		func() {
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				lastErr = fmt.Errorf("status code %d", resp.StatusCode)
				return
			}

			var data struct {
				IP       string `json:"ip"`
				Location struct {
					CountryCode string  `json:"country_code"`
					Latitude    float64 `json:"latitude"`
					Longitude   float64 `json:"longitude"`
				} `json:"location"`

				// us.ipapi.is (and some other regional endpoints) return flat fields:
				//   cc (country code), lat, lon (coordinates), etc.
				CC  string  `json:"cc"`
				Lat float64 `json:"lat"`
				Lon float64 `json:"lon"`
			}

			err = json.NewDecoder(resp.Body).Decode(&data)
			if err != nil {
				lastErr = err
				return
			}

			geo = &GeoInfo{
				IP: data.IP,
			}

			// Prefer the nested format when present; otherwise fall back to flat keys.
			if data.Location.CountryCode != "" || data.Location.Latitude != 0 || data.Location.Longitude != 0 {
				geo.CountryCode = data.Location.CountryCode
				geo.Location = fmt.Sprintf("%f,%f", data.Location.Latitude, data.Location.Longitude)
			} else {
				geo.CountryCode = data.CC
				geo.Location = fmt.Sprintf("%f,%f", data.Lat, data.Lon)
			}
		}()

		if geo != nil {
			return geo, nil
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("ipapi lookup failed (no response)")
	}
	return nil, lastErr
}

func getGeoInfoFromIpInfo(ip ...net.IP) (*GeoInfo, error) {
	url := "https://ipinfo.io/json"
	if len(ip) > 0 {
		url = fmt.Sprintf("https://ipinfo.io/%s/json", ip[0].String())
	}
	var geoInfo GeoInfo
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		IP      string `json:"ip"`
		Loc     string `json:"loc"`
		Country string `json:"country"`
	}

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	geoInfo.IP = data.IP
	geoInfo.CountryCode = data.Country
	geoInfo.Location = data.Loc

	return &geoInfo, nil
}
