package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaSVC "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	accessToken       string
	harborAPIBaseURL  = os.Getenv("HARBOR_API_BASE_URL")
	harborAPI2BaseURL = os.Getenv("HARBOR_API2_BASE_URL")
	iterableUpdateURL string
	lambdaClient      *lambdaSVC.Lambda
	pgDB              *sqlx.DB
	retryClient       *http.Client
)

type Params struct {
	IDFloor int64 `json:"idFloor"`
}

func handler(params Params) error {
	if len(accessToken) == 0 {
		authenticateAdminUser()
	}

	var results []struct {
		ID    int64   `db:"id"`
		Email *string `db:"email"`
	}
	if err := pgDB.Select(&results, selectQuery, params.IDFloor); err != nil {
		panic(fmt.Sprintf("failed to fetch user ids: %s", err))
	} else if len(results) == 0 {
		return nil
	}

	var users []map[string]interface{}
	for _, row := range results {
		weeklyFocus, err := getWeeklyFocusForUser(row.ID)
		if err != nil {
			fmt.Println(err)
			continue
		}
		user := map[string]interface{}{
			"userId":       fmt.Sprintf("%d", row.ID),
			"preferUserId": true,
			"dataFields":   map[string]*WeeklyFocus{"weeklyFocus": weeklyFocus},
		}
		if row.Email != nil {
			user["email"] = *row.Email
		}
		users = append(users, user)
	}

	if len(users) == 0 {
		return nil
	}

	body := map[string]interface{}{"users": users}
	b, _ := json.Marshal(body)

	resp, err := retryClient.Post(iterableUpdateURL, "application/json", bytes.NewBuffer(b))
	if err != nil {
		panic(fmt.Sprintf("error posting users: %s", err))
	}

	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		panic(fmt.Errorf("%d posting users: %s", resp.StatusCode, string(b)))
	}

	maxID := results[len(results)-1].ID
	if _, err := lambdaClient.Invoke(&lambdaSVC.InvokeInput{
		Payload:        []byte(fmt.Sprintf(`{"idFloor": %d}`, maxID)),
		FunctionName:   aws.String("IterableSyncWeeklyFocus"),
		InvocationType: aws.String("Event"),
	}); err != nil {
		panic(fmt.Sprintf("invocation failed for maxID(%d): %s", maxID, err))
	}

	return nil
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

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	}))
	lambdaClient = lambdaSVC.New(sess)

	apiKey := os.Getenv("ITERABLE_API_KEY")
	tmplt := "https://api.iterable.com/api/users/bulkUpdate?api_key=%s"
	iterableUpdateURL = fmt.Sprintf(tmplt, apiKey)
}

func main() {
	lambda.Start(handler)
}
