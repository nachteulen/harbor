package main

import (
    "encoding/xml"
    "fmt"
    "os"
    "net/http"
    "strings"
    "time"

    "github.com/aws/aws-lambda-go/lambda"
)

var baseUrl = "https://secure.shippingapis.com"
var path = "/ShippingAPI.dll"
var userID = os.Getenv("USPS_USER_ID")

var xmlParamTemplate = `
<CityStateLookupRequest USERID="%s">
	<ZipCode>
 		<Zip5>%s</Zip5>
 	</ZipCode>
</CityStateLookupRequest>
`

type GetCityStateRequest struct {
    Zipcode string `json:"zipcode"`
}

type USPSResponse struct {
    XMLName xml.Name `xml:"CityStateLookupResponse"`
    City string `xml:"ZipCode>City" json:"city"`
    State string `xml:"ZipCode>State" json:"state"`
}

type GetCityStateResponse struct {
    City string `json:"city"`
    State string `json:"state"`
}

func handler(gRR GetCityStateRequest) (*GetCityStateResponse, error) {
	if len(gRR.Zipcode) == 0 {
        return nil, fmt.Errorf("a zipcode is required")
    } else if len(gRR.Zipcode) != 5 {
        return nil, fmt.Errorf("invalid zipcode format: %s", gRR.Zipcode)
    }

	url := baseUrl + path
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/xml")

	q := req.URL.Query()
	q.Add("API", "CityStateLookup")
	q.Add("XML", fmt.Sprintf(xmlParamTemplate, userID, gRR.Zipcode))
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()

    r := USPSResponse{}
    err = xml.NewDecoder(resp.Body).Decode(&r)
    if err != nil {
        return nil, err
    }

    return &GetCityStateResponse{
        City: strings.Title(strings.ToLower(r.City)),
        State: r.State,
    }, nil
}

func main() {
    lambda.Start(handler)
}