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
	pgDB *sqlx.DB // TODO: connect to replica
)

type RowResult struct {
	Readiness float32 `db:"readiness"`
}

type RespBody struct {
	Rank      string  `json:"rank"`
	Readiness float32 `json:"readiness"`
}

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	userID := req.RequestContext.Authorizer["userID"].(string)

	var ownerships []int64
	oStr := req.RequestContext.Authorizer["allUserOwnershipsJSON"].(string)
	json.Unmarshal([]byte(oStr), &ownerships)

	query, args, _ := sqlx.In(query, ownerships)
	query = pgDB.Rebind(query)

	var result RowResult
	err := pgDB.Get(&result, query, args...)
	if err != nil {
		fmt.Printf("error getting risks readiness for user(%s): %s\n", userID, err)
		return &events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}

	var rank string
	switch {
	case result.Readiness < 0.03:
		rank = "New to this"
	case result.Readiness >= 0.03 && result.Readiness < 0.05:
		rank = "Getting going"
	case result.Readiness >= 0.05 && result.Readiness < 0.11:
		rank = "Feeling calm"
	case result.Readiness >= 0.11 && result.Readiness < 0.22:
		rank = "Hanging tough"
	case result.Readiness >= 0.22 && result.Readiness < 0.38:
		rank = "We got this"
	case result.Readiness >= 0.38 && result.Readiness < 0.56:
		rank = "Free and Fearless"
	case result.Readiness >= 0.56 && result.Readiness < 0.72:
		rank = "Feeling bold"
	case result.Readiness >= 0.72 && result.Readiness < 0.84:
		rank = "Not kidding around"
	case result.Readiness >= 0.84 && result.Readiness < 0.9:
		rank = "Jack-of-all-trades"
	case result.Readiness >= 0.9:
		rank = "Readiness Expert"
	}

	b, _ := json.Marshal(RespBody{Rank: rank, Readiness: result.Readiness})
	return &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(b),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
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
