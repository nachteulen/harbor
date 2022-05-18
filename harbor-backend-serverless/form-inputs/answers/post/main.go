package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
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
	PlanID     int64                   `json:"planID"`
	FormID     int64                   `json:"formID"`
	InputID    int64                   `json:"inputID"`
	Answer     string                  `json:"answer"`
	AnswerMeta *map[string]interface{} `json:"answerMeta"`
}

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	var reqBody ReqBody
	if err := json.Unmarshal([]byte(req.Body), &reqBody); err != nil {
		panic(fmt.Sprintf("unable to parse payload(%s): %s", req.Body, err))
	}

	if len(reqBody.Answer) == 0 {
		respB, _ := json.Marshal(map[string]string{"error": "missing `answer`"})
		return &events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       string(respB),
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}

	userID := req.RequestContext.Authorizer["userID"].(string)
	hhID := hhLib.GetCurrentHouseholdID(userID, rDB, pgDB)

	maxVersion := "1"
	v, ok := req.QueryStringParameters["maxPlanBuilderVersion"]
	if ok {
		maxVersion = v
	}

	args := []interface{}{
		reqBody.InputID,
		reqBody.Answer,
		reqBody.PlanID,
		reqBody.FormID,
		reqBody.Answer,
		maxVersion,
		reqBody.PlanID,
		hhID,
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

	args = append(args, hhID, hhID)

	var result struct {
		AnswerID      int64  `json:"answerID" db:"answer_id"`
		CurrentPoints int    `db:"current_plan_points"`
		AddedPoints   int    `db:"added_points"`
		MaxPoints     int    `db:"max_points"`
		PlanName      string `db:"plan_name"`
	}
	err := pgDB.Get(&result, query, args...)
	if err != nil {
		panic(fmt.Errorf("error saving answer for user(%s): %s", userID, err))
	}

	planProgress := float64(result.CurrentPoints+result.AddedPoints) / float64(result.MaxPoints)
	if planProgress > float64(1) {
		tmplt := "user(%s) progress(%f) exceeds(%d) for plan(%d) answer(%d)\n"
		fmt.Printf(tmplt, userID, planProgress, result.MaxPoints, reqBody.PlanID, result.AnswerID)
		planProgress = float64(1)
	}

	if planProgress == float64(1) {
		sendIterableEvent(userID, "PLAN_BUILDER_COMPLETED", result.PlanName)
	} else if result.CurrentPoints == 0 && result.AddedPoints != 0 {
		sendIterableEvent(userID, "PLAN_BUILDER_STARTED", result.PlanName)
	}

	b, _ := json.Marshal(map[string]interface{}{
		"answerID":               result.AnswerID,
		"globalProgressPlanPart": planProgress * 0.08,
	})
	return &events.APIGatewayProxyResponse{
		StatusCode: 201,
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
