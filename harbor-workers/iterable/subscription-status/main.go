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
	harborAPIBaseURL  string
	iterableUpdateURL string
	lambdaClient      *lambdaSVC.Lambda
	pgDB              *sqlx.DB
	retryClient       *http.Client
)

type UserRow struct {
	ID                       int64   `db:"id"`
	Email                    *string `db:"email"`
	Name                     *string `db:"name"`
	IsActive                 *bool   `db:"is_active"`
	IsTrial                  *bool   `db:"is_trial"`
	IsActiveCorporatePremium *bool   `db:"is_active_corporate_premium"`
}

type Params struct {
	IDFloor int64 `json:"idFloor"`
}

func handler(params Params) error {
	if len(accessToken) == 0 {
		authenticateAdminUser()
	}

	var results []*UserRow
	if err := pgDB.Select(&results, query, params.IDFloor); err != nil {
		panic(fmt.Sprintf("failed to fetch users: %s", err))
	} else if len(results) == 0 {
		return nil
	}

	var users []map[string]interface{}
	for _, row := range results {
		status, err := getUserSubscriptionStatus(row)
		if err != nil {
			fmt.Println(err)
			continue
		}

		user := map[string]interface{}{
			"userId":       fmt.Sprintf("%d", row.ID),
			"preferUserId": true,
			"dataFields":   map[string]string{"subscriptionStatus": status},
		}
		if row.Email != nil {
			user["email"] = *row.Email
		}
		users = append(users, user)
	}

	if len(users) != 0 {
		body := map[string]interface{}{"users": users}
		b, _ := json.Marshal(body)

		resp, err := retryClient.Post(iterableUpdateURL, "application/json", bytes.NewBuffer(b))
		if err != nil {
			panic(fmt.Sprintf("error posting users: %s", err))
		}

		if resp.StatusCode != 200 {
			b, _ := ioutil.ReadAll(resp.Body)
			return fmt.Errorf("%d posting users: %s", resp.StatusCode, string(b))
		}
	}

	maxID := results[len(results)-1].ID
	if _, err := lambdaClient.Invoke(&lambdaSVC.InvokeInput{
		Payload:        []byte(fmt.Sprintf(`{"idFloor": %d}`, maxID)),
		FunctionName:   aws.String("IterableSyncAppSubscriptions"),
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

	env := os.Getenv("ENVIRONMENT")
	harborAPIBaseURL = fmt.Sprintf("https://api.%s.helloharbor.com", env)

	apiKey := os.Getenv("ITERABLE_API_KEY")
	iterableUpdateURL = fmt.Sprintf("https://api.iterable.com/api/users/bulkUpdate?api_key=%s", apiKey)
}

func main() {
	lambda.Start(handler)
}
