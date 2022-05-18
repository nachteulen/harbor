package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	pgDB                 *sqlx.DB // TODO: connect to replica
	imageAssetPathBase   = "https://cdn.helloharbor.com"
	blobAssetPathTmplt   = imageAssetPathBase + "/%s-blob.png"
	circleAssetPathTmplt = imageAssetPathBase + "/%s-circle.png"
	listAssetPathTmplt   = imageAssetPathBase + "/%s.png"
)

type Risk struct {
	ID             int64   `db:"id" json:"id"`
	Name           string  `db:"name" json:"name"`
	Disclaimer     string  `db:"disclaimer" json:"disclaimer"`
	ListImage      string  `json:"listImagePath"`
	IconImage      string  `json:"iconImagePath"`
	CircleImage    string  `json:"circleImagePath"`
	IsPriority     bool    `db:"is_priority" json:"isPriority"`
	RiskLevel      *int    `db:"level_id" json:"riskLevel,omitempty"`
	RiskLevelText  *string `db:"level_text" json:"riskLevelText,omitempty"`
	RiskLevelColor *string `db:"level_color" json:"riskLevelColor,omitempty"`
	Readiness      float64 `db:"readiness" json:"readiness"`
	IsSubscribed   bool    `db:"is_subscribed" json:"isSubscribed"`
}

type RespBody struct {
	Count             int     `json:"count"`
	SubscribedCount   int     `json:"subscribedCount"`
	UnsubscribedCount int     `json:"unsubscribedCount"`
	Subscribed        []*Risk `json:"subscribed"`
	Unsubscribed      []*Risk `json:"unsubscribed"`
}

func handler(req events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	userID := req.RequestContext.Authorizer["userID"].(string)

	var ownerships []int64
	oStr := req.RequestContext.Authorizer["allUserOwnershipsJSON"].(string)
	json.Unmarshal([]byte(oStr), &ownerships)

	query, args, _ := sqlx.In(query, ownerships, ownerships)
	query = pgDB.Rebind(query)

	var results []*Risk
	err := pgDB.Select(&results, fmt.Sprintf(query, userID), args...)
	if err != nil {
		// TODO: retry
		panic(fmt.Sprintf("unable to get risks for user(%s): %s", userID, err))
	}

	if len(results) == 0 {
		fmt.Printf("no risks found for user(%s): %s\n", userID, err)
		return &events.APIGatewayProxyResponse{StatusCode: 404}, nil
	}

	return formatResponse(results), nil
}

func init() {
	d, err := sqlx.Connect("postgres", os.Getenv("DB_CONN"))
	if err != nil {
		panic(fmt.Sprintf("pg connection failed: %s", err))
	}
	pgDB = d
}

func main() {
	lambda.Start(handler)
}

func formatResponse(risks []*Risk) *events.APIGatewayProxyResponse {
	// initialize risks as the client chokes on `null`
	resp := RespBody{
		Count:        len(risks),
		Subscribed:   []*Risk{},
		Unsubscribed: []*Risk{},
	}

	for _, r := range risks {
		imgName := strings.ReplaceAll(r.Name, " ", "")

		r.IconImage = fmt.Sprintf(blobAssetPathTmplt, imgName)
		r.CircleImage = fmt.Sprintf(circleAssetPathTmplt, imgName)
		r.ListImage = fmt.Sprintf(listAssetPathTmplt, imgName)

		if r.IsSubscribed {
			resp.Subscribed = append(resp.Subscribed, r)
		} else {
			resp.Unsubscribed = append(resp.Unsubscribed, r)
		}
	}

	resp.SubscribedCount = len(resp.Subscribed)
	resp.UnsubscribedCount = len(resp.Unsubscribed)

	b, _ := json.Marshal(resp)
	return &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(b),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}
}
