package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

const beginSessionQuery = `
with inserted as (
	insert into analytics_events
	(type, correlation_id, user_id, meta)
	values ('BEGIN_SESSION', uuid_generate_v4(), $1, $2)
	returning correlation_id
)
select correlation_id from inserted`

func handleBeginSession(userID, sIP string) *events.APIGatewayProxyResponse {
	metaB, _ := json.Marshal(map[string]string{"ip": sIP})

	var correlationID string
	if err := pgDB.Get(
		&correlationID,
		beginSessionQuery,
		userID,
		metaB,
	); err != nil {
		panic(fmt.Errorf("unable to begin session for user(%s): %s", userID, err))
	}

	if env == "staging" || env == "production" {
		b, _ := json.Marshal(map[string]interface{}{
			"userId":     userID,
			"eventName":  "BEGIN_SESSION",
			"dataFields": map[string]string{"correlationID": correlationID},
		})
		_, err := retryClient.Post(iterableEventsURL, "application/json", bytes.NewBuffer(b))
		if err != nil {
			fmt.Printf("error posting event(%s) for user(%s): %s", string(b), userID, err)
		}
	}

	b, _ := json.Marshal(map[string]string{"correlationID": correlationID})
	return &events.APIGatewayProxyResponse{
		StatusCode: 201,
		Body:       string(b),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}
}
