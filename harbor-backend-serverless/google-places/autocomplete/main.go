package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-redis/redis/v8"
	"github.com/helloharbor/harbor-backend-serverless/google-places/lib"
)

const geoURL = "https://maps.googleapis.com/maps/api/place/autocomplete/json"

var (
	rDB        *redis.Client
	geoAPIKey  = os.Getenv("GEO_API_KEY")
	httpClient *http.Client
)

type AutoCompletionResponse struct {
	Predictions []*struct {
		ID          string `json:"place_id"`
		Description string `json:"description"`
		Meters      int64  `json:"distance_meters"`
	} `json:"predictions"`
}

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	userID := req.RequestContext.Authorizer["userID"].(string)

	text := req.QueryStringParameters["text"]
	if text == "" {
		return &events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}
	t, _ := url.QueryUnescape(text)

	origin := req.QueryStringParameters["origin"]
	if origin == "" {
		origin = "current"
	}

	getReq, _ := http.NewRequest("GET", geoURL, nil)
	q := getReq.URL.Query()
	q.Add("key", geoAPIKey)
	q.Add("input", t)
	q.Add("components", "country:USA")
	q.Add("radius", "50000")

	sess, err := getUserSessionToken(userID)
	if err != nil {
		fmt.Printf("error getting user(%s) session: %s\n", userID, err)
	} else {
		q.Add("sessiontoken", sess)
	}

	coords, err := lib.ParseOrigin(userID, origin, req.RequestContext.Identity.SourceIP)
	if err != nil {
		fmt.Printf("error parsing origin(%s) for user(%s): %s\n", origin, userID, err)
	} else {
		q.Add("location", fmt.Sprintf("%f,%f", coords.Lat, coords.Lng))
		q.Add("origin", fmt.Sprintf("%f,%f", coords.Lat, coords.Lng))
	}

	getReq.URL.RawQuery = q.Encode()
	resp, err := httpClient.Do(getReq)
	if err != nil {
		fmt.Printf("error getting(%s): %s\n", getReq.URL.String(), err)
		return &events.APIGatewayProxyResponse{StatusCode: 404}, nil
	}
	defer resp.Body.Close()

	var autoResp AutoCompletionResponse
	if err = json.NewDecoder(resp.Body).Decode(&autoResp); err != nil {
		b, _ := ioutil.ReadAll(resp.Body)
		tmplt := "unable to decode response(%s) for user(%s) %f,%f\n"
		fmt.Printf(tmplt, string(b), userID, coords.Lat, coords.Lng)
		return &events.APIGatewayProxyResponse{StatusCode: 404}, nil
	}

	sort.Slice(autoResp.Predictions, func(i, j int) bool {
		return autoResp.Predictions[i].Meters < autoResp.Predictions[j].Meters
	})

	results := []map[string]interface{}{}
	for _, p := range autoResp.Predictions {
		results = append(results, map[string]interface{}{
			"id":          p.ID,
			"description": p.Description,
			"miles":       float32(p.Meters) * 0.000621371,
		})
	}

	b, _ := json.Marshal(map[string]interface{}{
		"origin":  origin,
		"results": results,
	})
	return &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(b),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func init() {
	opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		fmt.Printf("unable to establish redis connection: %s\n", err)
	} else {
		rDB = redis.NewClient(opt)
	}

	httpClient = &http.Client{Timeout: 5 * time.Second}
}

func main() {
	lambda.Start(handler)
}
