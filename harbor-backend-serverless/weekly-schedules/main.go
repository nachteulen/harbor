package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	pgDB *sqlx.DB
)

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	var userID string

	uID, ok := req.RequestContext.Authorizer["userID"]
	if ok {
		userID = uID.(string)
	} else {
		var reqBody struct {
			UserID int64 `json:"userID"`
		}
		if err := json.Unmarshal([]byte(req.Body), &reqBody); err != nil {
			panic(fmt.Errorf("unable to parse payload(%s): %s", req.Body, err))
		}
		userID = fmt.Sprintf("%d", reqBody.UserID)
	}

	_, err := pgDB.Exec(query, userID)
	if err != nil {
		panic(fmt.Errorf("unable to create weekly schedule for user(%s): %s", userID, err))
	}

	return &events.APIGatewayProxyResponse{StatusCode: 204}, nil
}

func init() {
	d, err := sqlx.Connect("postgres", os.Getenv("DB_CONN"))
	if err != nil {
		panic(err)
	}
	pgDB = d
}

func main() {
	lambda.Start(handler)
}
