package main

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

const endSessionQuery = `
with found as (
	select created_at, correlation_id
	from analytics_events
	where correlation_id = $1 and type = 'BEGIN_SESSION'
), inserted as (
	insert into analytics_events
	(type, correlation_id, user_id)
	select 'END_SESSION' as type, correlation_id, $2 as user_id
	from found where correlation_id is not null
	returning created_at
) select extract(seconds from
	(select created_at from inserted) - (select created_at from found)
) as session_length`

const retryEndSessionQuery = `
with open_sessions as (
	select
		user_id,
		created_at,
		correlation_id,
		count(*) over()
	from analytics_events ae1
	where
		type = 'BEGIN_SESSION'
		and user_id = $1
		and (extract(epoch from now() - created_at) / 3600) < 24
		and not exists (
			select id
			from analytics_events ae2
			where ae1.correlation_id = ae2.correlation_id and type = 'END_SESSION')
), open_session as (
	select correlation_id, user_id, created_at
	from open_sessions
	where count = 1
), inserted as (
	insert into analytics_events
	(type, correlation_id, user_id)
	select 'END_SESSION' as type, correlation_id, user_id
	from open_session
	returning created_at
), extracted as (
	select extract(seconds from
		(select created_at from inserted) - (select created_at from open_session)
	) as session_length
)
select * from extracted where session_length is not null`

func handleEndSession(userID, correlationID string) *events.APIGatewayProxyResponse {
	var result *float64
	if err := pgDB.Get(&result, endSessionQuery, correlationID, userID); err != nil {
		fmt.Printf("unable to end session for user(%s), correlationID(%s): %s\n", userID, correlationID, err)
		if err := pgDB.Get(&result, retryEndSessionQuery, userID); err != nil {
			fmt.Printf("unable to retry end session for user(%s): %s\n", userID, err)
			return &events.APIGatewayProxyResponse{StatusCode: 204}
		}
	}

	if result == nil {
		fmt.Printf("null session length for user(%s), correlationID(%s)\n", userID, correlationID)
	} else if env == "staging" || env == "production" {
		b, _ := json.Marshal(map[string]interface{}{
			"userId":    userID,
			"eventName": "END_SESSION",
			"dataFields": map[string]interface{}{
				"correlationID": correlationID,
				"sessionLength": *result,
			},
		})
		_, err := retryClient.Post(iterableEventsURL, "application/json", bytes.NewBuffer(b))
		if err != nil {
			fmt.Printf("error posting event(%s) for user(%s): %s", string(b), userID, err)
		}
	}

	return &events.APIGatewayProxyResponse{StatusCode: 201}
}
