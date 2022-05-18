package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/go-redis/redis/v8"

	log "github.com/sirupsen/logrus"

	"weather-events-get/models"
)

var (
	redisConn *redis.Client
	stdFields map[string]interface{}

	ctx       = context.Background()
	traceID = ""
)

func handler(awsCtx context.Context, req events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	id := req.PathParameters["id"]
	setCtxFields(awsCtx)

	cachedVal, err := getCachedEvent(id)
	if err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"error": err}).
			Fatal("failed getting cached event")
	} else if cachedVal == nil {
		return &events.APIGatewayProxyResponse{StatusCode: 404}, nil
	}

	resp, err := hydrateAlertResponse(*cachedVal)
	if err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"error": err}).
			Fatal("failed to hydrate alert response")
	}
	byteResp, err := json.Marshal(resp)

	log.WithFields(stdFields).WithFields(log.Fields{"resp": resp}).Info()

	return &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(byteResp),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func getCachedEvent(alertID string) (*string, error) {
	res, err := redisConn.Get(ctx, alertID).Result()
	if err == redis.Nil {
		log.WithFields(stdFields).WithFields(log.Fields{"alertID": alertID}).
			Warn("cache miss")
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("unexpected error during redis fetch(%s), %s", alertID, err)
	}

	return &res, nil
}

func hydrateAlertResponse(cachedVal string) (*models.WeatherEventResponse, error) {
	incomingMsg := models.RedisAlertMsg{}
	err := json.Unmarshal([]byte(cachedVal), &incomingMsg)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshall incoming alert\n %s", cachedVal)
	}

	// check to see if event code refers to the national weather service
	// if yes, get the code from the NWS event code value
	eventCode := incomingMsg.Info.EventCode.Value
	codeMapping, ok := models.AlertCodeToCategorization[eventCode]
	if !ok {
		return nil, fmt.Errorf("code %s does not exist in cateogry mapping", eventCode)
	}

	alertResp := models.WeatherEventResponse{}
	alertResp.ID = incomingMsg.Identifier
	alertResp.RefIds = incomingMsg.References
	alertResp.Status = incomingMsg.Status
	alertResp.Categorization = models.WeatherEventCategorization{
		Text: codeMapping.Text,
		Category: string(codeMapping.Category),
		Code: eventCode,
		Level: string(codeMapping.Level),
	}
	alertResp.Headline = incomingMsg.Info.Headline
	alertResp.Description = incomingMsg.Info.Description
	alertResp.Instructions = incomingMsg.Info.Instruction
	alertResp.Polygon, err = extractCoordsFromPolygon(incomingMsg.Info.Area.Polygon)
	if err != nil {
		return nil, err
	}
	alertResp.OnsetTime, err = getUTC3339(incomingMsg.Info.Onset)
	if err != nil {
		return nil, fmt.Errorf("could not parse onset time(%s), %s", incomingMsg.Info.Onset, err)
	}
	alertResp.ExpirationTime, err = getUTC3339(incomingMsg.Info.Expires)
	if err != nil {
		return nil, fmt.Errorf("could not parse expiration time(%s), %s", incomingMsg.Info.Onset, err)
	}

	return &alertResp, nil
}

func getUTC3339(alertTimeMsg string) (string, error){
	isoTime, err := time.Parse(time.RFC3339, alertTimeMsg)
	if err != nil {
		return "", err
	}
	formatted := isoTime.UTC().Format("2006-01-02T15:04:05.000Z0700")

	return formatted, nil
}

func extractCoordsFromPolygon(polygonStr string) ([]models.WeatherEventCoords, error) {
	// prefer to have this test in the function
	if polygonStr == "" {return nil, nil}

	var coordArrayResp []models.WeatherEventCoords
	strArrCoords := strings.Split(polygonStr, " ")

	for _, coord := range strArrCoords {
		coordSet := strings.Split(coord, ",")
		if len(coordSet) != 2 {
			return nil, fmt.Errorf("polygon format not valid (%s)", polygonStr)
		}
		coordResp := models.WeatherEventCoords{
			Lat: coordSet[0],
			Long: coordSet[1],
		}
		coordArrayResp = append(coordArrayResp, coordResp)
	}

	return coordArrayResp, nil
}

func setCtxFields(awsCtx context.Context) {
	lCtx, ok := lambdacontext.FromContext(awsCtx)

	if ok {
		traceID = lCtx.AwsRequestID
	}
	stdFields = log.Fields{"traceID": traceID}
}

func init() {
	opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		panic(fmt.Errorf("unable to connect to redis: %s", err))
	}
	redisConn = redis.NewClient(opt)

	log.SetLevel(log.DebugLevel)
	log.SetFormatter(&log.JSONFormatter{
		DisableTimestamp : true,
	})
	log.SetOutput(os.Stdout)
}

func main() {
	lambda.Start(handler)
}
