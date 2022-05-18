package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/helloharbor/harbor-backend-serverless/google-places/lib"
)

const geoURL = "https://maps.googleapis.com/maps/api/place/nearbysearch/json"

var (
	geoAPIKey           = os.Getenv("GEO_API_KEY")
	httpClient          *http.Client
	supportedPlaceTypes = map[string]bool{
		"gas_station": true,
	}
)

type NearbyPlaces struct {
	Name     string `json:"name"`
	Address  string `json:"vicinity"`
	Geometry struct {
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

	getReq, _ := http.NewRequest("GET", geoURL, nil)
	q := getReq.URL.Query()
	q.Add("key", geoAPIKey)

	var coords *lib.CoordinatePair
	origin := req.QueryStringParameters["origin"]
	if strings.Contains(origin, ",") {
		originParts := strings.Split(origin, ",")
		if len(originParts) != 2 {
			panic(fmt.Errorf("cannot parse origin(%s)", origin))
		}
		nLat, err := strconv.ParseFloat(originParts[0], 64)
		if err != nil {
			panic(fmt.Errorf("cannot parse latitude(%s)", originParts[0]))
		}
		nLng, err := strconv.ParseFloat(originParts[1], 64)
		if err != nil {
			panic(fmt.Errorf("cannot parse longitude(%s)", originParts[1]))
		}
		coords = &lib.CoordinatePair{nLat, nLng}
	} else {
		if origin == "" {
			origin = "current"
		}
		var err error
		sIP := req.RequestContext.Identity.SourceIP
		coords, err = lib.ParseOrigin(userID, origin, sIP)
		if err != nil {
			tmplt := "error parsing origin(%s) for user(%s): %s"
			panic(fmt.Errorf(tmplt, origin, userID, err))
		}
	}
	q.Add("location", fmt.Sprintf("%f,%f", coords.Lat, coords.Lng))

	var typeOrKeyword bool
	var placeType string
	if supportedPlaceTypes[req.QueryStringParameters["type"]] {
		placeType = req.QueryStringParameters["type"]
		q.Add("type", placeType)
		typeOrKeyword = true
	}

	keyword, ok := req.QueryStringParameters["keyword"]
	k, _ := url.QueryUnescape(keyword)

	// Adding `keyword` and `type` with same value can yield `ZERO_RESULTS`.
	if ok && k != placeType {
		q.Add("keyword", k)
		typeOrKeyword = true
	}
	if typeOrKeyword {
		q.Add("rankby", "distance")
	} else {
		q.Add("radius", "50000")
	}

	getReq.URL.RawQuery = q.Encode()
	resp, err := httpClient.Do(getReq)
	if err != nil {
		panic(fmt.Errorf("error getting(%s): %s", getReq.URL.String(), err))
	}
	defer resp.Body.Close()

	var nearResp struct {
		Results []*NearbyPlaces `json:"results"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&nearResp); err != nil {
		b, _ := ioutil.ReadAll(resp.Body)
		tmplt := "unable to decode response(%s) for user(%s) %f,%f"
		panic(fmt.Errorf(tmplt, string(b), userID, coords.Lat, coords.Lng))
	}

	results := []map[string]interface{}{}
	for _, p := range nearResp.Results {
		results = append(results, map[string]interface{}{
			"name":      p.Name,
			"address":   p.Address,
			"latitude":  p.Geometry.Location.Lat,
			"longitude": p.Geometry.Location.Lng,
			"miles": milesBetween(
				coords.Lat,
				coords.Lng,
				p.Geometry.Location.Lat,
				p.Geometry.Location.Lng,
			),
		})
	}

	// results could/should already be sorted, but double check in case
	sort.Slice(results, func(i, j int) bool {
		return results[i]["miles"].(float64) < results[j]["miles"].(float64)
	})

	b, _ := json.Marshal(map[string]interface{}{
		"results":         results,
		"origin":          origin,
		"originLatitude":  coords.Lat,
		"originLongitude": coords.Lng,
	})
	return &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(b),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func init() {
	httpClient = &http.Client{Timeout: 5 * time.Second}
}

func main() {
	lambda.Start(handler)
}
