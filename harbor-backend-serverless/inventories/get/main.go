package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	pgDB  *sqlx.DB
	query = "select product_links from inventories where id = $1"
)

type RowResult struct {
	ProductLinks *string `db:"product_links"`
}

type ProductLink struct {
	Link string `json:"link"`
}

type RespBody struct {
	ProductLinks []string `json:"productLinks"`
}

func handler(req events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	id := req.PathParameters["id"]
	nID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		fmt.Printf("error parsing inventory id(%s): %s\n", id, err)
		return &events.APIGatewayProxyResponse{StatusCode: 401}, nil
	}

	var result RowResult
	err = pgDB.Get(&result, query, nID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &events.APIGatewayProxyResponse{StatusCode: 404}, nil
		}
		fmt.Printf("error getting inventory(%d): %s\n", nID, err)
		return &events.APIGatewayProxyResponse{StatusCode: 500}, nil
	}

	var productLinks []*ProductLink
	if result.ProductLinks != nil {
		err = json.Unmarshal([]byte(*result.ProductLinks), &productLinks)
		if err != nil {
			fmt.Printf("error parsing product_links for inventory(%d): %s\n", nID, err)
		}
	}

	var links []string
	for _, l := range productLinks {
		links = append(links, l.Link)
	}

	b, _ := json.Marshal(RespBody{ProductLinks: links})
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
