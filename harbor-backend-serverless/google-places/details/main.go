package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-redis/redis/v8"
)

const geoURL = "https://maps.googleapis.com/maps/api/place/details/json"

var (
	rDB        *redis.Client
	geoAPIKey  = os.Getenv("GEO_API_KEY")
	httpClient *http.Client
)

type PlaceDetailsResp struct {
	AddressComponents []struct {
		LongName  string   `json:"long_name"`
		ShortName string   `json:"short_name"`
		Types     []string `json:"types"`
	} `json:"address_components"`
	FormattedAddress string `json:"formatted_address"`
	Geometry         struct {
		Location struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"location"`
	} `json:"geometry"`
}

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	userID := req.RequestContext.Authorizer["userID"].(string)

	id := req.PathParameters["id"]
	if id == "" {
		return &events.APIGatewayProxyResponse{StatusCode: 400}, nil
	}

	getReq, _ := http.NewRequest("GET", geoURL, nil)
	q := getReq.URL.Query()
	q.Add("key", geoAPIKey)
	q.Add("place_id", id)
	q.Add("fields", "address_components,formatted_address,geometry")

	sess, err := getUserSessionToken(userID)
	if err != nil {
		fmt.Printf("error getting user(%s) session: %s\n", userID, err)
	} else {
		q.Add("sessiontoken", sess)
	}

	getReq.URL.RawQuery = q.Encode()
	resp, err := httpClient.Do(getReq)
	if err != nil {
		tmplt := "error getting(%s) for user(%s): %s"
		panic(fmt.Errorf(tmplt, getReq.URL.String(), userID, err))
	}
	defer resp.Body.Close()

	var detailsResp struct {
		Result PlaceDetailsResp `json:"result"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&detailsResp); err != nil {
		b, _ := ioutil.ReadAll(resp.Body)
		tmplt := "unable to decode place(%s) response(%s) for user(%s)"
		panic(fmt.Errorf(tmplt, id, string(b), userID))
	}

	place := map[string]interface{}{
		"address":   detailsResp.Result.FormattedAddress,
		"latitude":  detailsResp.Result.Geometry.Location.Lat,
		"longitude": detailsResp.Result.Geometry.Location.Lng,
		"validated": true,
	}

	for _, c := range detailsResp.Result.AddressComponents {
		if len(c.Types) == 0 {
			continue
		}

		if c.Types[0] == "locality" {
			place["city"] = c.LongName
		} else if c.Types[0] == "postal_code" {
			place["zipcode"] = c.LongName
		} else if c.Types[0] == "country" {
			place["countryName"] = c.ShortName
		} else if c.Types[0] == "administrative_area_level_1" {
			place["stateName"] = c.LongName
		}
	}

	b, _ := json.Marshal(place)
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
