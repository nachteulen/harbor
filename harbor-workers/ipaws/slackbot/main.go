package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/helloharbor/harbor-workers/ipaws/shared/models"
	"github.com/jmoiron/sqlx"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	lambdaContext "github.com/aws/aws-lambda-go/lambdacontext"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

var (
	pgDB      *sqlx.DB
	redisConn *redis.Client
	stdFields map[string]interface{}

	ctx         = context.Background()
	redisUrl    = os.Getenv("REDIS_URL")
	environment = os.Getenv("SLACKBOT_ENV")
	traceID     = ""
)

type extRespProperties struct {
	headline string
}

func handler(awsCtx context.Context, req events.SNSEvent) error {
	if environment != "development" {
		log.WithFields(stdFields).
			Info("slackbot not running in development, functionality disabled")
		return nil
	}
	log.WithFields(stdFields).Info("ipaws slackbot entry point")

	setCtxFields(awsCtx)
	snsMsgBody := req.Records[0].SNS.Message

	var alertObj = models.ShortAlertMsg{}
	err := json.Unmarshal([]byte(snsMsgBody), &alertObj)
	if err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"alert-body": alertObj, "error": err}).
			Fatal("failed unmarshall alert-obj")
	}

	body := getReqBody(alertObj)
	slackBody, err := json.Marshal(*body)
	if err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"slack-body": slackBody, "error": err}).
			Fatal("could not marshall slack body")
	}
	log.WithFields(stdFields).WithFields(log.Fields{"slack-body": slackBody}).Info("sending slack message")

	// send to the generic channel always
	allLocations := models.CityData{
		Name: "All",
		URL:  "https://hooks.slack.com/services/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",
		Lat:  0.0,
		Long: 0.0,
	}
	err = sendSlackReq(allLocations, slackBody)
	if err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"slack-body": slackBody, "error": err}).
			Fatal("slack request failed")
	}

	return nil
}

func sendSlackReq(cd models.CityData, reqBody []byte) error {
	slackReq, err := http.NewRequest(http.MethodPost, cd.URL, bytes.NewBuffer(reqBody))
	if err != nil {
		return fmt.Errorf("failed to create POST req")
	}
	slackReq.Header.Add("Content-Type", "application/text")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(slackReq)
	if err != nil {
		return fmt.Errorf("request failed")
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	if buf.String() != "ok" {
		return fmt.Errorf("unexpected response from Slack: %s", buf.String())
	}

	return nil
}

func getReqBody(alertObj models.ShortAlertMsg) *models.SlackRequestBody {
	//need to get extended properties form redis
	ep, err := getExtProps(alertObj.Identifier)
	if err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"alert-body": alertObj, "error": err}).
			Fatal("failed getting ext properites")
	}
	users, err := fetchUsersInRange(alertObj.BoundingBox)
	var numUsersStr string
	if users != nil {
		numUsersStr = fmt.Sprintf("num users: %d\n%v", len(*users), *users)
	} else {
		numUsersStr = "geocode only"
	}

	return &models.SlackRequestBody{
		Blocks: []models.Block{
			{
				Type: "header",
				Sections: models.Section{
					Type:  "plain_text",
					Text:  alertObj.Categorization.Level + "-" + alertObj.Categorization.Category,
					Emoji: true,
				},
			},
			{
				Type: "section",
				Sections: models.Section{
					Type: "plain_text",
					Text: alertObj.Categorization.Text,
				},
			},
			{
				Type: "section",
				Sections: models.Section{
					Type: "plain_text",
					Text: ep.headline,
				},
			},
			{
				Type: "section",
				Sections: models.Section{
					Type: "plain_text",
					Text: alertObj.Identifier + "\n" + alertObj.Polygon,
				},
			},
			{
				Type: "section",
				Sections: models.Section{
					Type: "plain_text",
					Text: numUsersStr,
				},
			},
		},
	}
}

func getExtProps(alertID string) (*extRespProperties, error) {
	log.WithFields(stdFields).WithFields(log.Fields{"alertID": alertID, "redis_conn": os.Getenv("REDIS_URL")}).Info("sending redis message")

	fullAlert, err := redisConn.Get(ctx, alertID).Result()
	if err == redis.Nil {
		fmt.Printf("key(%s) not found in cache", fullAlert)
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("unexpected error during redis fetch(%s), %s", alertID, err)
	}

	fullObj := models.AlertMsg{}
	json.Unmarshal([]byte(fullAlert), &fullObj)

	return &extRespProperties{
		headline: fullObj.Info.Headline,
	}, nil
}

func fetchUsersInRange(boundingBox string) (*[]models.User, error) {
	ptSplit := strings.Split(boundingBox, " ")
	if len(ptSplit) <= 0 || ptSplit[0] == "" {
		log.WithFields(stdFields).Info("No bounding box")
		return nil, nil
	}
	if len(ptSplit) != 4 {
		return nil, fmt.Errorf("boundingBox not in correct format, found %d points, req 4", len(ptSplit))
	}
	latL, _ := strconv.ParseFloat(ptSplit[0], 64)
	latH, _ := strconv.ParseFloat(ptSplit[1], 64)
	lngL, _ := strconv.ParseFloat(ptSplit[2], 64)
	lngH, _ := strconv.ParseFloat(ptSplit[3], 64)

	var res []*models.UserRow
	if err := pgDB.Select(&res, query, latL, latH, lngL, lngH); err != nil {
		return nil, fmt.Errorf("fetchUsersInRange query failed: %s", err)
	}

	var uArr []models.User
	for _, v := range res {
		uArr = append(uArr,
			models.User{
				Email:     *v.Email,
				Address:   *v.Address,
				Latitude:  *v.Latitude,
				Longitude: *v.Longitude})
	}
	log.WithFields(stdFields).
		WithFields(log.Fields{"latL": latL, "latH": latH, "lngL": lngL, "lngH": lngH,
			"num_users": len(res), "users": fmt.Sprintf("%+v", uArr)}).
		Info("filtered users")

	return &uArr, nil
}

func setCtxFields(awsCtx context.Context) {
	lambdaCtx, ok := lambdaContext.FromContext(awsCtx)

	if ok {
		traceID = lambdaCtx.AwsRequestID
	}
	stdFields = log.Fields{"traceID": traceID}
}

func init() {
	if environment == "development" {
		opt, _ := redis.ParseURL(redisUrl)
		redisConn = redis.NewClient(opt)

		ckEnv := os.Getenv("REDIS_RO_DB_KEY")
		dbConnStr, err := redisConn.Get(ctx, ckEnv).Result()
		if err != nil {
			panic(err)
		}

		d, err := sqlx.Connect("postgres", dbConnStr)
		if err != nil {
			panic(err)
		}
		pgDB = d

		log.SetLevel(log.DebugLevel)
		log.SetFormatter(&log.JSONFormatter{
			DisableTimestamp: true,
		})
		log.SetOutput(os.Stdout)
	}
}

func main() {
	lambda.Start(handler)
}
