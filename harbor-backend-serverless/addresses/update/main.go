package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaSVC "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	pgDB                 *sqlx.DB
	retryClient          *http.Client
	lambdaClient         *lambdaSVC.Lambda
	harborRiskProfileURL = os.Getenv("HARBOR_RISK_PROFILE_URL")
)

type ReqBody struct {
	State   *string  `json:"state"`
	Zipcode *string  `json:"zipcode"`
	Lat     *float64 `json:"lat"`
	Lng     *float64 `json:"lng"`
}

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	userID := req.RequestContext.Authorizer["userID"].(string)

	var body ReqBody
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		panic(fmt.Sprintf("unable to parse payload(%s): %s", req.Body, err))
	}

	if body.Lat != nil && body.Lng != nil {
		if err := handleGeoUpdate(*body.Lat, *body.Lng, userID); err != nil {
			panic(fmt.Errorf("unable to handle geo update: %s", err))
		}
	} else if body.Zipcode != nil && body.State != nil {
		if err := handleZipUpdate(*body.State, *body.Zipcode, userID); err != nil {
			panic(fmt.Sprintf("unable to handle zip update: %s\n", err))
		}
	} else {
		fmt.Printf("not enough info to update address with profile data")
		return &events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}

	payload, _ := json.Marshal(events.APIGatewayProxyRequest{
		Body: fmt.Sprintf(`{"userID": %s}`, userID),
	})
	if _, err := lambdaClient.Invoke(&lambdaSVC.InvokeInput{
		Payload:        payload,
		FunctionName:   aws.String("WeeklyScheduleUpsert"),
		InvocationType: aws.String("Event"),
	}); err != nil {
		fmt.Printf("weekly upsert invocation failed for user(%s): %s\n", userID, err)
	}

	return &events.APIGatewayProxyResponse{StatusCode: 200}, nil
}

func init() {
	d, err := sqlx.Connect("postgres", os.Getenv("DB_CONN"))
	if err != nil {
		panic(fmt.Sprintf("pg connection failed: %s", err))
	}
	pgDB = d

	lambdaClient = lambdaSVC.New(session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	})))

	rC := retryablehttp.NewClient()
	rC.Logger = nil
	rC.RetryMax = 3
	retryClient = rC.StandardClient()
	retryClient.Timeout = 10 * time.Second
}

func main() {
	lambda.Start(handler)
}
