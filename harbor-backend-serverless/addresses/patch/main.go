package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
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
	"github.com/mmcloughlin/geohash"
)

var (
	pgDB           *sqlx.DB
	retryClient    *http.Client
	lambdaClient   *lambdaSVC.Lambda
	riskProfileURL = os.Getenv("HARBOR_GEO_RISK_PROFILE_URL")
)

type ReqBody struct {
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Address *string `json:"address"`
	Zipcode *string `json:"zipcode"`
}

type LocalAuthority struct {
	Name    string  `json:"name"`
	Address string  `json:"address"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Type    string  `json:"type"`
}

type HazardResponse struct {
	LocalAuthorities []*LocalAuthority `json:"localAuthorities"`
	Profile          map[string]int    `json:"profile"`
}

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	userID := req.RequestContext.Authorizer["userID"].(string)
	oStr := req.RequestContext.Authorizer["allUserOwnershipsJSON"].(string)

	var body ReqBody
	if err := json.Unmarshal([]byte(req.Body), &body); err != nil {
		panic(fmt.Sprintf("unable to parse payload(%s): %s", req.Body, err))
	}

	if body.Lat == 0 || body.Lng == 0 {
		tmplt := "invalid lat/lng %f,%f for user(%s) address update\n"
		fmt.Printf(tmplt, body.Lat, body.Lng, userID)
		return &events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}

	hhReq, _ := http.NewRequest("GET", riskProfileURL, nil)
	q := hhReq.URL.Query()
	q.Add("lat", fmt.Sprintf("%f", body.Lat))
	q.Add("lng", fmt.Sprintf("%f", body.Lng))
	hhReq.URL.RawQuery = q.Encode()

	resp, err := retryClient.Do(hhReq)
	if err != nil {
		tmplt := "unable to get %f,%f profile for user(%s): %s"
		panic(fmt.Errorf(tmplt, body.Lat, body.Lng, userID, err))
	}
	defer resp.Body.Close()

	var hazardResp HazardResponse
	if err = json.NewDecoder(resp.Body).Decode(&hazardResp); err != nil {
		b, _ := ioutil.ReadAll(resp.Body)
		tmplt := "%s: unable to decode %f,%f profile for user(%s): %s"
		panic(fmt.Errorf(tmplt, err, body.Lat, body.Lng, userID, string(b)))
	}

	profileID := geohash.EncodeIntWithPrecision(body.Lat, body.Lng, 64)
	profile, err := insertRiskProfile(
		userID,
		req.PathParameters["addressID"],
		oStr,
		profileID,
		&hazardResp,
		body.Lat,
		body.Lng,
		body.Address,
		body.Zipcode,
	)
	if err != nil {
		tmplt := "unable to insert %f,%f profile(%+v) for user(%s): %s"
		panic(fmt.Errorf(tmplt, body.Lat, body.Lng, hazardResp.Profile, userID, err))
	}

	upsertWeeklySchedule(userID)

	b, _ := json.Marshal(map[string]interface{}{"highRisk": profile})
	return &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(b),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
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
