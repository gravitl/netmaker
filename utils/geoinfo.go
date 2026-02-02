package utils

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type GeoInfo struct {
	IP          string
	CountryCode string
	Location    string
}

// GetGeoInfo returns the ip, location and country code of the host it's called on.
func GetGeoInfo() (*GeoInfo, error) {
	geoInfo, err := getGeoInfoFromIPAPI()
	if err == nil {
		return geoInfo, nil
	}

	geoInfo, err = getGeoInfoFromCloudFlare()
	if err == nil {
		return geoInfo, nil
	}

	return getGeoInfoFromIpInfo()
}

func getGeoInfoFromIPAPI() (*GeoInfo, error) {
	resp, err := http.Get("https://api.ipapi.is")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data struct {
		IP       string `json:"ip"`
		Location struct {
			CountryCode string `json:"country_code"`
			Latitude    string `json:"latitude"`
			Longitude   string `json:"longitude"`
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
		Location:    data.Location.Latitude + "," + data.Location.Longitude,
		CountryCode: data.Location.CountryCode,
	}, nil
}

func getGeoInfoFromCloudFlare() (*GeoInfo, error) {
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

func getGeoInfoFromIpInfo() (*GeoInfo, error) {
	var geoInfo GeoInfo
	resp, err := http.Get("https://ipinfo.io/json")
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
