package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	iterableEventsURL string
	pgDB              *sqlx.DB
	retryClient       *http.Client
	env               = os.Getenv("ENVIRONMENT")
)

const query = `
insert into analytics_events
(type, user_id, correlation_id)
values ($1, $2, $3)`

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	userID := req.RequestContext.Authorizer["userID"].(string)

	if req.Path == "/analytics-events/begin-session" {
		sIP := req.RequestContext.Identity.SourceIP
		return handleBeginSession(userID, sIP), nil
	}

	var body struct {
		CID  string  `json:"correlationID"`
		Type *string `json:"type"`
	}
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		panic(fmt.Errorf("unable to parse payload(%s): %s", req.Body, err))
	}

	if req.Path == "/analytics-events/end-session" {
		return handleEndSession(userID, body.CID), nil
	}

	if body.Type == nil {
		fmt.Printf("missing type for body: %+v\n", body)
		return &events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}

	args := []interface{}{
		*body.Type,
		userID,
	}
	if body.CID == "" {
		args = append(args, nil)
	} else {
		args = append(args, body.CID)
	}

	_, err := pgDB.Exec(query, args...)
	if err != nil {
		panic(fmt.Errorf("unable to save event(%+v): %s", body, err))
	}

	return &events.APIGatewayProxyResponse{StatusCode: 201}, nil
}

func init() {
	d, err := sqlx.Connect("postgres", os.Getenv("DB_CONN"))
	if err != nil {
		panic(fmt.Sprintf("pg connection failed: %s", err))
	}
	pgDB = d

	rC := retryablehttp.NewClient()
	rC.Logger = nil
	rC.RetryMax = 3
	retryClient = rC.StandardClient()
	retryClient.Timeout = 5 * time.Second

	apiKey := os.Getenv("ITERABLE_API_KEY")
	iterableEventsURL = fmt.Sprintf("https://api.iterable.com/api/events/track?api_key=%s", apiKey)
}

func main() {
	lambda.Start(handler)
}
