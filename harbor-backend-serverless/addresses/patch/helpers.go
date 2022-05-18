package main

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go/aws"
	lambdaSVC "github.com/aws/aws-sdk-go/service/lambda"
)

type ProfileItem struct {
	RiskID    int     `db:"risk_id" json:"riskID"`
	RiskLevel *int    `db:"level_id" json:"riskLevel"`
	RiskName  string  `db:"name" json:"riskName"`
	RiskText  *string `db:"risk_text" json:"riskText"`
	RiskColor *string `db:"risk_color" json:"riskColor"`
}

func insertRiskProfile(
	userID,
	addressID,
	oStr string,
	idParam uint64,
	hazardResp *HazardResponse,
	lat,
	lng float64,
	address,
	zipcode *string,
) ([]*ProfileItem, error) {
	var profile []map[string]int
	for k, v := range hazardResp.Profile {
		nRiskID, err := strconv.Atoi(k)
		if err != nil {
			tmplt := "invalid riskID(%s) for user(%s) profile(%+v)\n"
			fmt.Printf(tmplt, k, userID, hazardResp.Profile)
			continue
		}

		profile = append(profile, map[string]int{
			"risk_id":  nRiskID,
			"level_id": v,
		})
	}

	if len(profile) != 12 {
		return nil, fmt.Errorf("invalid profile(%d): %+v", idParam, hazardResp.Profile)
	}

	profileB, _ := json.Marshal(profile)
	args := []interface{}{idParam, profileB}

	if len(hazardResp.LocalAuthorities) == 0 {
		args = append(args, nil)
	} else {
		authoritiesB, _ := json.Marshal(hazardResp.LocalAuthorities)
		args = append(args, authoritiesB)
	}

	args = append(args, lat, lng)
	if address != nil {
		args = append(args, *address)
	}
	if zipcode != nil {
		args = append(args, *zipcode)
	}
	args = append(args, addressID, oStr, userID)

	var highRisks []*ProfileItem
	if err := pgDB.Select(&highRisks, query, args...); err != nil {
		tmplt := "unable to insert/update user(%s) address with profile(%s): %s"
		return nil, fmt.Errorf(tmplt, userID, string(profileB), err)
	}

	return highRisks, nil
}

func upsertWeeklySchedule(userID string) {
	payload, _ := json.Marshal(events.APIGatewayProxyRequest{
		Body: fmt.Sprintf(`{"userID": %s}`, userID),
	})
	if _, err := lambdaClient.Invoke(&lambdaSVC.InvokeInput{
		Payload:        payload,
		FunctionName:   aws.String("WeeklyScheduleUpsert"),
		InvocationType: aws.String("Event"),
	}); err != nil {
		fmt.Printf("weekly upsert invocation failed for user(%s): %s\n", userID, err)
	}
}
