package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/go-redis/redis/v8"

	log "github.com/sirupsen/logrus"

	"weather-events-get-all/models"
)

var (
	redisConn *redis.Client
	stdFields map[string]interface{}

	ctx            = context.Background()
	allEventsQName = os.Getenv("REDIS_ALL_EVENTS_KEY")
)

func handler(awsCtx context.Context, req events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	setCtxFields(awsCtx)

	lat, long, err := getCoordinates(req.QueryStringParameters["latlong"])
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       fmt.Sprint(err),
			Headers:    map[string]string{"Content-Type": "application/json"},
		}, nil
	}

	cachedAlerts, err := getCachedAlerts()
	if err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"err": err}).Fatal("getCachedAlerts failed")
	}

	foundEvents, err := findEvents(*lat, *long, cachedAlerts)
	if err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"err": err}).Fatal("findEvents failed")
	}

	resp, err := json.Marshal(*foundEvents)
	if err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"err": err}).Fatal("failed unmarshalling events")
	}

	return &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(resp),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func getCachedAlerts() (*[]models.WeatherAlert, error) {
	strArray, err := redisConn.LRange(ctx, allEventsQName, 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("lrange failed: %s", err)
	}

	var alertMsgArr []models.WeatherAlert
	alertMsg := models.WeatherAlert{}
	for _, str := range strArray {
		err := json.Unmarshal([]byte(str), &alertMsg)
		if err != nil {
			return nil, err
		}
		alertMsgArr = append(alertMsgArr, alertMsg)
	}

	return &alertMsgArr, nil
}

func findEvents(lat float64, long float64, alerts *[]models.WeatherAlert) (*[]models.WeatherEventsResponse, error) {
	resp := []models.WeatherEventsResponse{}

	for _, alert := range *alerts {
		pStr := alert.BoundingBox
		if pStr != "" {
			bbr, err := GetBoundingBoxFromString(alert.BoundingBox)
			if err != nil {
				return nil, err
			}
			if bbr == nil {
				log.WithFields(stdFields).Info("no bounding box")
				return &resp, nil
			}
			if bbr.ContainsPoint(lat, long) {
				wer := models.WeatherEventsResponse{
					ID:     alert.Identifier,
					RefIds: alert.RefIds,
					Categorization: models.WeatherEventCategorization{
						Text:     alert.Categorization.Text,
						Category: alert.Categorization.Category,
						Code:     alert.Categorization.Code,
						Level:    alert.Categorization.Level,
					},
					OnsetTime:      alert.OnsetTime,
					ExpirationTime: alert.ExpirationTime,
				}

				resp = append(resp, wer)
			}
		}
	}

	return &resp, nil
}

func getCoordinates(qParam string) (*float64, *float64, error) {
	if qParam == "" {
		return nil, nil, fmt.Errorf("query param latlong missing")
	}
	ll := strings.Split(qParam, ",")
	if len(ll) != 2 {
		return nil, nil, fmt.Errorf("query param latlong incorrect format")
	}
	lat, errLat := strconv.ParseFloat(ll[0], 64)
	long, errLong := strconv.ParseFloat(ll[1], 64)
	if errLat != nil || errLong != nil {
		return nil, nil, fmt.Errorf("query param latlong incorrect format")
	}

	return &lat, &long, nil
}

func setCtxFields(awsCtx context.Context) {
	lCtx, ok := lambdacontext.FromContext(awsCtx)
	reqID := ""

	if ok {
		reqID = lCtx.AwsRequestID
	}
	stdFields = log.Fields{"reqID": reqID}
}

func init() {
	opt, _ := redis.ParseURL(os.Getenv("REDIS_URL"))
	redisConn = redis.NewClient(opt)
}

func main() {
	lambda.Start(handler)
}
