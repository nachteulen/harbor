package main

import (
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	pgDB             *sqlx.DB
	retryClient      *http.Client
	documentsURL     = os.Getenv("DOCUMENTS_URL")
	safeLocationsURL = os.Getenv("SAFE_LOCATIONS_URL")
)

type RespBody struct {
	Docs             []map[string]interface{} `json:"documents"`
	Members          []map[string]interface{} `json:"members"`
	Contacts         []map[string]interface{} `json:"emergencyContacts"`
	SafeLocations    []map[string]interface{} `json:"safeLocations"`
	LocalAuthorities []map[string]interface{} `json:"localAuthorities"`
}

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	var respBody RespBody
	var wg sync.WaitGroup

	wg.Add(1)
	go getDocsAPIData(&respBody, &wg, &req)

	wg.Add(1)
	go getSafeLocationsAPIData(&respBody, &wg, &req)

	wg.Add(1)
	go getDBData(&respBody, &wg, &req)

	wg.Wait()

	b, _ := json.Marshal(respBody)

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

	rC := retryablehttp.NewClient()
	rC.Logger = nil
	rC.RetryMax = 3
	retryClient = rC.StandardClient()
	retryClient.Timeout = 5 * time.Second
}

func main() {
	lambda.Start(handler)
}
