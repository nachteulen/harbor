package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/go-redis/redis/v8"
	geo "github.com/helloharbor/harbor-workers/ipaws/shared/geometries"
	"github.com/helloharbor/harbor-workers/ipaws/shared/models"
	"github.com/jmoiron/sqlx"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	lambdaContext "github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/hashicorp/go-retryablehttp"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
)

var (
	iterableTWURL string
	pgDB          *sqlx.DB
	redisConn     *redis.Client
	retryClient   *http.Client
	snsClient     *sns.SNS
	stdFields     map[string]interface{}

	ctx = context.Background()

	awsRegion   = os.Getenv("AWS_REGION")
	redisUrl    = os.Getenv("REDIS_URL")
	environment = os.Getenv("LAMBDA_ENV")
	snsArn      = os.Getenv("NOTIFICATIONS_SNS_ARN")
)

type UserRow struct {
	Email     *string `db:"email"`
	Address   *string `db:"address"`
	Latitude  *string `db:"latitude"`
	Longitude *string `db:"longitude"`
}

type User struct {
	Email     string
	Address   string
	Latitude  string
	Longitude string
}

func handler(awsCtx context.Context, req events.SNSEvent) error {
	setCtxFields(awsCtx)

	snsMsgBody := req.Records[0].SNS.Message
	var alertObj = models.ShortAlertMsg{}
	err := json.Unmarshal([]byte(snsMsgBody), &alertObj)
	if err != nil {
		log.WithFields(stdFields).
			WithFields(log.Fields{"alert-obj": alertObj, "err": err}).Fatal("failed to extract message from sns")
	}

	users, err := fetchUsersInRange(alertObj.BoundingBox)
	if err != nil {
		log.WithFields(stdFields).
			WithFields(log.Fields{"boundingBox": alertObj.BoundingBox, "err": err}).Fatal("failed to fetch users in range")
	}

	if users != nil {
		err = sendToNotificationSNS(alertObj, users)
		if err != nil {
			log.WithFields(stdFields).
				WithFields(log.Fields{"err": err}).Warn("failed to place beacon users data to SNS topic")
		}

		triggerAlerts, err := createTriggerAlerts(alertObj, *users)
		if err != nil {
			log.WithFields(stdFields).
				WithFields(log.Fields{"alert": alertObj, "err": err}).Fatal("failed to create triggers")
		}

		if triggerAlerts != nil && environment != "development" {
			err = invokeIterableWorkflows(*triggerAlerts)
			if err != nil {
				log.WithFields(stdFields).
					WithFields(log.Fields{"triggerAlerts": *triggerAlerts, "err": err}).
					Fatal("failed to send iterable notifications")
			}
		}
	}

	return nil
}

func invokeIterableWorkflows(triggers []models.WorkflowTriggerAlert) error {
	for _, trigger := range triggers {

		tBuff, err := json.Marshal(trigger)
		if err != nil {
			return fmt.Errorf("failed to marshal object %+v: %s", trigger, err)
		}

		log.WithFields(stdFields).WithFields(log.Fields{"url": iterableTWURL, "payload": bytes.NewBuffer(tBuff)}).
			Info("sending payload to iterable")

		resp, err := retryClient.Post(iterableTWURL, "application/json", bytes.NewBuffer(tBuff))
		if err != nil {
			return fmt.Errorf("failed to send message %s: %s", string(tBuff), err)
		}

		if resp.StatusCode != 200 {
			b, _ := ioutil.ReadAll(resp.Body)
			return fmt.Errorf("failed to post workflow (%d: %s)", resp.StatusCode, string(b))
		}
	}

	return nil
}

func fetchUsersInRange(boundingBox string) (*[]User, error) {
	bbr, err := geo.GetBoundingBoxFromString(boundingBox)
	if err != nil {
		return nil, fmt.Errorf("get bounding box failed: %s", err)
	}
	if bbr == nil {
		log.WithFields(stdFields).Info("No bounding box")
		return nil, nil
	}

	var res []*UserRow
	if err := pgDB.Select(&res, query, bbr.LatLo, bbr.LatHi, bbr.LngLo, bbr.LngHi); err != nil {
		return nil, fmt.Errorf("fetchUsersInRange query failed: %s", err)
	}

	var uArr []User
	for _, v := range res {
		uArr = append(uArr,
			User{
				Email:     *v.Email,
				Address:   *v.Address,
				Latitude:  *v.Latitude,
				Longitude: *v.Longitude})
	}
	log.WithFields(stdFields).
		WithFields(log.Fields{"bounding box": fmt.Sprintf("%+v", *bbr),
			"num_users": len(res), "users": fmt.Sprintf("%+v", uArr)}).
		Info("filtered users")

	return &uArr, nil
}

func sendToNotificationSNS(msg models.ShortAlertMsg, users *[]User) error {
	var userStrs []string

	for _, user := range *users {
		userStrs = append(userStrs, user.Email)
	}

	nm := models.NotificationMessage{
		Identifier: msg.Identifier,
		RefIds:     msg.RefIds,
		Users:      userStrs,
		Polygon:    msg.Polygon,
		OnsetTime:  msg.OnsetTime,
	}

	usersMsg, err := json.Marshal(nm)
	if err != nil {
		return err
	}
	usersMsgStr := string(usersMsg)

	fmt.Printf("usersMsgStr: %s\nsnsArn: %s\n", usersMsgStr, snsArn)

	_, err = snsClient.Publish(&sns.PublishInput{
		Message:  &usersMsgStr,
		TopicArn: &snsArn,
	})

	if err != nil {
		return fmt.Errorf("failed sending sns\nmessage: %s\nsnsArn: %s", usersMsgStr, snsArn)
	}

	return nil
}

func createTriggerAlerts(msg models.ShortAlertMsg, users []User) (*[]models.WorkflowTriggerAlert, error) {
	var triggerAlerts []models.WorkflowTriggerAlert
	currOutLvl, found := models.OutlookLevelDict[msg.Categorization.Level]
	if !found {
		return nil, fmt.Errorf("failed to lookup outlook level: %s", msg.Categorization.Level)
	}
	var prevOutLvl = "0"
	if msg.IsUpdate && msg.RefIds != nil && len(msg.RefIds) > 0 {
		var pErr error
		prevOutLvl, pErr = GetPreviousOutlookLevel(msg)
		if pErr != nil {
			return nil, pErr
		}
	}

	for _, user := range users {
		ta := models.WorkflowTriggerAlert{
			Name:  "BEACON",
			Email: user.Email,
			DataFields: models.BeaconFields{
				PushData: models.BeaconPush{
					Update:               msg.IsUpdate,
					AlertId:              msg.Identifier,
					AlertType:            msg.Categorization.Text,
					CurrentOutlookLevel:  currOutLvl,
					PreviousOutlookLevel: prevOutLvl,
					WeatherEvent:         msg.Categorization.Category,
					Deeplink:             "emergency",
				},
			},
		}

		triggerAlerts = append(triggerAlerts, ta)
	}

	return &triggerAlerts, nil
}

func GetPreviousOutlookLevel(msg models.ShortAlertMsg) (string, error) {
	var prevOutLvl = "0"
	lastStamp := time.Date(1971, time.November, 1, 1, 1, 0, 0, time.UTC)
	for _, refId := range msg.RefIds {
		fullAlert, err := redisConn.Get(ctx, refId).Result()
		if err == redis.Nil {
			log.WithFields(stdFields).Warningf("key(%s) not found in cache", refId)
			return "", nil
		} else if err != nil {
			return "", fmt.Errorf("unexpected error during redis fetch(%s), %s", refId, err)
		}

		fullObj := models.AlertMsg{}

		err = json.Unmarshal([]byte(fullAlert), &fullObj)
		if err == nil {
			return "", nil
		}

		msgDate, err := time.Parse(time.RFC3339, msg.OnsetTime)
		if err != nil {
			return "", fmt.Errorf("failed parsing %s: %s", msg.OnsetTime, err)
		}
		if msgDate.After(lastStamp) {
			lastStamp = msgDate
			code := models.AlertCodeToCategorization[fullObj.Info.EventCode.Value]
			prevOutLvl = models.OutlookLevelDict[string(code.Level)]
		}
	}

	return prevOutLvl, nil
}

func setCtxFields(awsCtx context.Context) {
	lambdaCtx, ok := lambdaContext.FromContext(awsCtx)
	reqID := ""

	if ok {
		reqID = lambdaCtx.AwsRequestID
	}
	stdFields = log.Fields{"reqID": reqID}
}

func init() {
	opt, _ := redis.ParseURL(redisUrl)
	redisConn = redis.NewClient(opt)

	var dbConnStr string
	if environment == "development" {
		ckEnv := os.Getenv("REDIS_RO_DB_KEY")
		cs, err := redisConn.Get(ctx, ckEnv).Result()
		if err != nil {
			panic(err)
		}
		dbConnStr = cs
	} else {
		dbConnStr = os.Getenv("DB_CONN")
	}
	d, err := sqlx.Connect("postgres", dbConnStr)
	if err != nil {
		panic(err)
	}
	pgDB = d

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))
	snsClient = sns.New(sess)

	apiKey := os.Getenv("ITERABLE_API_KEY")
	iterableBaseAPIURL := "https://api.iterable.com/api"
	tpl := iterableBaseAPIURL + "/events/track?api_key=%s"
	iterableTWURL = fmt.Sprintf(tpl, apiKey)

	rC := retryablehttp.NewClient()
	rC.Logger = nil
	rC.RetryMax = 3
	retryClient = rC.StandardClient()
	retryClient.Timeout = 5 * time.Second
	retryClient.Timeout = 5 * time.Second
}

func main() {
	lambda.Start(handler)
}
