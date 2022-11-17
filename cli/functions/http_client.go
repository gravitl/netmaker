package functions

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

func Request[T any](method, route string, payload any) *T {
	requestURL := "http://localhost:3000"
	var (
		req *http.Request
		err error
	)
	if payload == nil {
		req, err = http.NewRequest(method, requestURL+route, nil)
	} else {
		payloadBytes, jsonErr := json.Marshal(payload)
		if jsonErr != nil {
			log.Fatalf("Error in request JSON marshalling: %s", err)
		}
		req, err = http.NewRequest(method, requestURL+route, bytes.NewReader(payloadBytes))
	}
	if err != nil {
		log.Fatalf("Client could not create request: %s", err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Client error making http request: %s", err)
	}

	resBodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Fatalf("Client could not read response body: %s", err)
	}
	body := new(T)
	if err := json.Unmarshal(resBodyBytes, body); err != nil {
		log.Fatalf("Error unmarshalling JSON: %s", err)
	}
	return body
}
