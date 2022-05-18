package main

import (
	"database/sql"
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
	chapterID := req.PathParameters["id"]
	oStr := req.RequestContext.Authorizer["allUserOwnershipsJSON"].(string)

	var result string
	err := pgDB.Get(&result, query, oStr, chapterID, chapterID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &events.APIGatewayProxyResponse{StatusCode: 404}, nil
		}
		tmplt := "error getting chapter(%s) for user(%v): %s"
		panic(fmt.Errorf(tmplt, chapterID, req.RequestContext.Authorizer["userID"], err))
	}

	return &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       result,
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
