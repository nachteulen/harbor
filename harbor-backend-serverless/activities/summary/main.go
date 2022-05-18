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
	ID        int64   `db:"id" json:"id"`
	GID       int     `db:"activity_group_id" json:"activityGroupID"`
	LID       int     `db:"activity_level_id" json:"activityLevelID"`
	Theme     string  `db:"theme" json:"theme"`
	Icon      string  `db:"icon_image_path" json:"iconImagePath"`
	Readiness float32 `db:"readiness" json:"readiness"`
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

	var results []*RowResult
	err := pgDB.Select(&results, query, args...)
	if err != nil {
		tmplt := "error getting risks activites for user(%+v): %s\n"
		fmt.Printf(tmplt, userID, err)
		return &events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}

	b, _ := json.Marshal(results)
	return &events.APIGatewayProxyResponse{StatusCode: 200, Body: string(b)}, nil
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
