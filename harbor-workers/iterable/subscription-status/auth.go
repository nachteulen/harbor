package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type AuthResponse struct {
	AccessToken string `json:"accessToken"`
}

func authenticateAdminUser() {
	uName := os.Getenv("ADMIN_USERNAME")
	pWord := os.Getenv("ADMIN_PASSWORD")
	tmplt := `{"email": "%s", "password": "%s"}`

	payload := []byte(fmt.Sprintf(tmplt, uName, pWord))
	url := harborAPIBaseURL + "/v1/sessions/native"

	resp, err := retryClient.Post(url, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		panic(fmt.Errorf("unable to authenticate admin user: %s", err))
	}
	defer resp.Body.Close()
	if resp.StatusCode != 201 {
		b, _ := ioutil.ReadAll(resp.Body)
		panic(fmt.Sprintf("error authenticating: %s", string(b)))
	}

	var authResponse AuthResponse
	if err = json.NewDecoder(resp.Body).Decode(&authResponse); err != nil {
		panic(fmt.Errorf("unable to parse auth response: %s", err))
	}
	accessToken = authResponse.AccessToken
}
