package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var allowedOverrides = []string{
	"subscriptionStatus",
}

var (
	iterableUpdateURL string
	pgDB              *sqlx.DB
	retryClient       *http.Client
)

type Params struct {
	UserID    int64                  `json:"userID"`
	Overrides map[string]interface{} `json:"overrides"`
}

func handler(params Params) error {
	var userData UserData
	if err := pgDB.Get(&userData, query, params.UserID); err != nil {
		if err == sql.ErrNoRows {
			fmt.Printf("no results found for user(%d)\n", params.UserID)
			return nil
		}
		panic(fmt.Sprintf("pg query failed for user(%d): %s", params.UserID, err))
	}

	dataFields := userData.toDataFields()

	for _, o := range allowedOverrides {
		v, ok := params.Overrides[o]
		if ok {
			dataFields[o] = v
		}
	}

	iterableUser := map[string]interface{}{
		"preferUserId": true,
		"userId":       fmt.Sprintf("%d", userData.ID),
		"dataFields":   dataFields,
	}

	if userData.Email != nil {
		iterableUser["email"] = *userData.Email
	}

	b, _ := json.Marshal(iterableUser)
	resp, err := retryClient.Post(iterableUpdateURL, "application/json", bytes.NewBuffer(b))
	if err != nil {
		panic(fmt.Errorf("error posting user(%d): %s", userData.ID, err))
	}

	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		panic(fmt.Errorf("%d posting user(%d): %s", resp.StatusCode, userData.ID, string(b)))
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

	apiKey := os.Getenv("ITERABLE_API_KEY")
	iterableUsersBaseURL := "https://api.iterable.com/api/users"

	tmplt := iterableUsersBaseURL + "/update?api_key=%s"
	iterableUpdateURL = fmt.Sprintf(tmplt, apiKey)
}

func main() {
	lambda.Start(handler)
}
