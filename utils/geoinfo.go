package utils

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
)

type GeoInfo struct {
	IP          string
	CountryCode string
	Location    string
}

// GetGeoInfo returns the ip, location and country code of the host it's called on.
func GetGeoInfo(ip ...net.IP) (*GeoInfo, error) {
	geoInfo, err := getGeoInfoFromIPAPI(ip...)
	if err == nil {
		return geoInfo, nil
	}

	geoInfo, err = getGeoInfoFromCloudFlare(ip...)
	if err == nil {
		return geoInfo, nil
	}

	return getGeoInfoFromIpInfo(ip...)
}

func getGeoInfoFromIPAPI(ip ...net.IP) (*GeoInfo, error) {
	url := "https://api.ipapi.is"
	if len(ip) > 0 {
		url = fmt.Sprintf("https://api.ipapi.is/?q=%s", ip[0].String())
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		IP       string `json:"ip"`
		Location struct {
			CountryCode string  `json:"country_code"`
			Latitude    float64 `json:"latitude"`
			Longitude   float64 `json:"longitude"`
		} `json:"location"`
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}

	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	return &GeoInfo{
		IP:          data.IP,
		Location:    fmt.Sprintf("%f,%f", data.Location.Latitude, data.Location.Longitude),
		CountryCode: data.Location.CountryCode,
	}, nil
}

func getGeoInfoFromCloudFlare(ip ...net.IP) (*GeoInfo, error) {
	var geoInfo GeoInfo
	resp, err := http.Get("https://speed.cloudflare.com/meta")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respMap := make(map[string]interface{})
	err = json.NewDecoder(resp.Body).Decode(&respMap)
	if err != nil {
		return nil, err
	}

	_, ok := respMap["clientIp"]
	if ok {
		geoInfo.IP = respMap["clientIp"].(string)
	}

	_, ok = respMap["country"]
	if ok {
		geoInfo.CountryCode = respMap["country"].(string)
	}

	var latitude, longitude string
	_, ok = respMap["latitude"]
	if ok {
		latitude = respMap["latitude"].(string)
	}

	_, ok = respMap["longitude"]
	if ok {
		longitude = respMap["longitude"].(string)
	}

	if latitude != "" && longitude != "" {
		geoInfo.Location = latitude + "," + longitude
	}

	return &geoInfo, nil
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
