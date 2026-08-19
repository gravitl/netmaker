package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
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
	hosts := []string{
		"us.ipapi.is",
		"de.ipapi.is",
		"sg.ipapi.is",
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	client := &http.Client{}

	resultCh := make(chan *GeoInfo, 1)
	errCh := make(chan error, len(hosts))
	doneCh := make(chan struct{})

	var wg sync.WaitGroup
	wg.Add(len(hosts))

	for _, host := range hosts {
		host := host

		go func() {
			defer wg.Done()

			url := fmt.Sprintf("https://%s/", host)

			if len(ip) > 0 && ip[0] != nil {
				url = fmt.Sprintf(
					"https://%s/?q=%s",
					host,
					ip[0].String(),
				)
			}

			req, err := http.NewRequestWithContext(
				ctx,
				http.MethodGet,
				url,
				nil,
			)
			if err != nil {
				errCh <- fmt.Errorf("%s: %w", host, err)
				return
			}

			resp, err := client.Do(req)
			if err != nil {
				if ctx.Err() != nil {
					return
				}

				errCh <- fmt.Errorf("%s: %w", host, err)
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				errCh <- fmt.Errorf(
					"%s returned status code %d",
					host,
					resp.StatusCode,
				)
				return
			}

			var data struct {
				IP string `json:"ip"`

				Location struct {
					CountryCode string   `json:"country_code"`
					Latitude    *float64 `json:"latitude"`
					Longitude   *float64 `json:"longitude"`
				} `json:"location"`

				CC  string   `json:"cc"`
				Lat *float64 `json:"lat"`
				Lon *float64 `json:"lon"`
			}

			if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
				errCh <- fmt.Errorf(
					"%s: decode response: %w",
					host,
					err,
				)
				return
			}

			geo := &GeoInfo{
				IP: data.IP,
			}

			if data.Location.Latitude != nil &&
				data.Location.Longitude != nil {

				geo.CountryCode = data.Location.CountryCode
				geo.Location = fmt.Sprintf(
					"%f,%f",
					*data.Location.Latitude,
					*data.Location.Longitude,
				)

			} else if data.Lat != nil && data.Lon != nil {

				geo.CountryCode = data.CC
				geo.Location = fmt.Sprintf(
					"%f,%f",
					*data.Lat,
					*data.Lon,
				)
			}

			if !geo.HasUsableGeo() {
				errCh <- fmt.Errorf(
					"%s returned no usable geo",
					host,
				)
				return
			}

			select {
			case resultCh <- geo:
			default:
			}
		}()
	}

	go func() {
		wg.Wait()
		close(doneCh)
	}()

	select {
	case geo := <-resultCh:
		cancel()
		wg.Wait()
		return geo, nil

	case <-doneCh:
		cancel()

		select {
		case geo := <-resultCh:
			return geo, nil
		default:
		}

		var lastErr error

		for {
			select {
			case err := <-errCh:
				lastErr = err

			default:
				if lastErr == nil {
					lastErr = fmt.Errorf(
						"ipapi lookup failed: no usable geo",
					)
				}

				return nil, lastErr
			}
		}

	case <-ctx.Done():
		// A result might have arrived at almost the same time as the timeout.
		select {
		case geo := <-resultCh:
			wg.Wait()
			return geo, nil
		default:
		}

		var lastErr error

		for {
			select {
			case err := <-errCh:
				lastErr = err

			default:
				if lastErr != nil {
					return nil, fmt.Errorf(
						"ipapi lookup timed out: %w",
						lastErr,
					)
				}

				return nil, fmt.Errorf(
					"ipapi lookup timed out",
				)
			}
		}
	}
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
