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
	Avg   float32 `db:"avg"`
	Count int     `db:"count"`
}

type RespBody struct {
	AverageReadiness float32 `json:"averageReadiness"`
	RisksCount       int     `json:"risksCount"`
}

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	userID := req.RequestContext.Authorizer["userID"].(string)

	var ownerships []int64
	oStr := req.RequestContext.Authorizer["allUserOwnershipsJSON"].(string)
	json.Unmarshal([]byte(oStr), &ownerships)

	query, args, _ := sqlx.In(query, ownerships, ownerships)
	query = pgDB.Rebind(query)

	var result RowResult
	err := pgDB.Get(&result, query, args...)
	if err != nil {
		fmt.Printf("error getting risks summary for user(%s): %s\n", userID, err)
		return &events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}

	b, _ := json.Marshal(RespBody{
		AverageReadiness: result.Avg,
		RisksCount:       result.Count,
	})
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
