package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/go-retryablehttp"
	hhLib "github.com/helloharbor/harbor-backend-serverless/households/lib"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	pgDB             *sqlx.DB
	rDB              *redis.Client
	retryClient      *http.Client
	iterableEventURL = os.Getenv("ITERABLE_EVENT_URL")
)

type ReqBody struct {
	Answer     string                  `json:"answer"`
	AnswerMeta *map[string]interface{} `json:"answerMeta"`
}

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	answerID := req.PathParameters["answerID"]
	userID := req.RequestContext.Authorizer["userID"].(string)
	hhID := hhLib.GetCurrentHouseholdID(userID, rDB, pgDB)

	var reqBody ReqBody
	if err := json.Unmarshal([]byte(req.Body), &reqBody); err != nil {
		panic(fmt.Errorf("unable to parse payload(%s): %s", req.Body, err))
	}

	if !strings.Contains(req.Path, "/plans") {
		return legacyUpdate(answerID, userID, hhID, &reqBody)
	}

	maxVersion := "1"
	v, ok := req.QueryStringParameters["maxPlanBuilderVersion"]
	if ok {
		maxVersion = v
	}

	args := []interface{}{
		answerID,
		hhID,
		reqBody.Answer,
		reqBody.Answer,
		maxVersion,
		req.PathParameters["planID"],
		hhID,
		userID,
	}

	if reqBody.AnswerMeta != nil {
		// TODO: add validation to prevent future parsing errors
		b, _ := json.Marshal(*reqBody.AnswerMeta)
		args = append(args, b)
	} else {
		args = append(args, nil)
	}

	var result struct {
		CurrentPoints int    `db:"current_plan_points"`
		AddedPoints   int    `db:"added_points"`
		MaxPoints     int    `db:"max_points"`
		PlanName      string `db:"plan_name"`
	}
	if err := pgDB.Get(
		&result,
		query,
		args...,
	); err != nil {
		tmplt := "unable to update answer(%s) for user(%s): %s"
		panic(fmt.Errorf(tmplt, answerID, userID, err))
	}

	planProgress := float64(result.CurrentPoints+result.AddedPoints) / float64(result.MaxPoints)
	if planProgress > float64(1) {
		tmplt := "user(%s) progress(%f) exceeds(%d) for plan(%s) answer(%s)\n"
		fmt.Printf(tmplt, userID, planProgress, result.MaxPoints, req.PathParameters["planID"], answerID)
		planProgress = float64(1)
	}

	if planProgress == float64(1) {
		sendIterableEvent(userID, "PLAN_BUILDER_COMPLETED", result.PlanName)
	}

	b, _ := json.Marshal(map[string]interface{}{
		"globalProgressPlanPart": planProgress * 0.08,
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

	opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		panic(err)
	}
	rDB = redis.NewClient(opt)

	rC := retryablehttp.NewClient()
	rC.Logger = nil
	rC.RetryMax = 2
	retryClient = rC.StandardClient()
	retryClient.Timeout = 5 * time.Second
}

func main() {
	lambda.Start(handler)
}
