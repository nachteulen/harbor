package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

func sendIterableEvent(userID, eventName, planName string) {
	if os.Getenv("ENVIRONMENT") == "development" {
		return
	}

	eventData := map[string]interface{}{
		"userId":     userID,
		"eventName":  eventName,
		"dataFields": map[string]interface{}{"planName": planName},
	}

	b, _ := json.Marshal(eventData)
	resp, err := retryClient.Post(iterableEventURL, "application/json", bytes.NewBuffer(b))
	if err != nil {
		tmplt := "error posting user(%s) event(%s) for plan(%s): %s\n"
		fmt.Printf(tmplt, userID, eventName, planName, err)
		return
	}

	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		tmplt := "%d posting user(%s) event(%s) for plan(%s): %s\n"
		fmt.Printf(tmplt, resp.StatusCode, userID, eventName, planName, string(b))
	}
}
