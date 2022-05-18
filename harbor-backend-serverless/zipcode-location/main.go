package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	uspsURL = os.Getenv("USPS_URL")
	uspsUID = os.Getenv("USPS_USER_ID")
)

var query = `
select city, state_abbr
from zipcode_locations
where id = $1`

var xmlParamTemplate = `
<CityStateLookupRequest USERID="%s">
	<ZipCode>
 		<Zip5>%s</Zip5>
 	</ZipCode>
</CityStateLookupRequest>
`

type USPSResponse struct {
	XMLName xml.Name `xml:"CityStateLookupResponse"`
	City    string   `xml:"ZipCode>City" json:"city"`
	State   string   `xml:"ZipCode>State" json:"state"`
}

type RespBody struct {
	City  string `db:"city" json:"city"`
	State string `db:"state_abbr" json:"stateAbbr"`
}

// N.B. currently this endpoint is only called internally (configured as a
// APIGatewayProxyRequest though to better monitor errors). Should external clients
// ever need to call this endpoint a better authorization scheme should be in place.
func handler(req events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	authToken, ok := req.PathParameters["authToken"]
	if !ok || len(authToken) == 0 {
		fmt.Println("missing auth")
		return makeE(401, "E_MISSING_AUTH"), nil
	} else if authToken != os.Getenv("INTERNAL_AUTH_TOKEN") {
		fmt.Println("missing auth")
		return makeE(401, "E_INVALID_AUTH"), nil
	}

	zip, ok := req.PathParameters["zipcode"]
	if !ok || len(zip) == 0 {
		fmt.Println("missing zipcode")
		return makeE(400, "E_INVALID_REQUEST"), nil
	}

	if len(zip) != 5 {
		return makeE(400, "E_INVALID_ZIPCODE_FORMAT"), nil
	}

	nZip, err := strconv.Atoi(zip)
	if err != nil {
		fmt.Printf("invalid zipcode: %s\n", zip)
		return makeE(400, "E_INVALID_ZIPCODE"), nil
	}

	db, err := sqlx.Connect("postgres", os.Getenv("DB_CONN"))
	if err != nil {
		fmt.Printf("unable to connect to db for zip(%s): %s\n", zip, err)
	} else {
		var result RespBody
		if err := db.Get(&result, query, nZip); err != nil {
			fmt.Printf("unable to get info for zip(%s): %s\n", zip, err)
		} else {
			b, _ := json.Marshal(result)
			return &events.APIGatewayProxyResponse{StatusCode: 200, Body: string(b)}, nil
		}
	}

	uspsReq, err := http.NewRequest("GET", uspsURL, nil)
	if err != nil {
		return nil, err
	}
	uspsReq.Header.Add("Content-Type", "application/xml")

	q := uspsReq.URL.Query()
	q.Add("API", "CityStateLookup")
	q.Add("XML", fmt.Sprintf(xmlParamTemplate, uspsUID, zip))
	uspsReq.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(uspsReq)
	if err != nil {
		fmt.Printf("usps error for zipcode(%s): %s", zip, err)
		return makeE(500, "E_API_ERROR"), nil
	}
	defer resp.Body.Close()

	var uspsResp USPSResponse
	err = xml.NewDecoder(resp.Body).Decode(&uspsResp)
	if err != nil {
		fmt.Printf("usps invalid response for zipcode(%s): %s", zip, err)
		return makeE(500, "E_API_RESPONSE"), nil
	}

	b, _ := json.Marshal(RespBody{
		City:  strings.Title(strings.ToLower(uspsResp.City)),
		State: uspsResp.State,
	})
	return &events.APIGatewayProxyResponse{StatusCode: 200, Body: string(b)}, nil
}

func main() {
	lambda.Start(handler)
}

func makeE(statusCode int, msg string) *events.APIGatewayProxyResponse {
	return &events.APIGatewayProxyResponse{
		Headers:    map[string]string{"Content-Type": "application/json"},
		StatusCode: statusCode,
		Body:       fmt.Sprintf(`{ "error": "%s" }`, msg),
	}
}
