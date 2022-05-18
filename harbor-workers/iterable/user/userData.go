package main

import (
	"encoding/json"
	"fmt"
)

type UserData struct {
	ID            int64   `db:"id"`
	FirstName     *string `db:"first_name"`
	LastName      *string `db:"last_name"`
	Email         *string `db:"email"`
	Zipcode       *string `db:"zipcode"`
	City          *string `db:"city"`
	State         *string `db:"state"`
	Subscriptions *string `db:"subscribed_risks"`
	Profile       *string `db:"risk_profile"`
}

func (u *UserData) toDataFields() map[string]interface{} {
	dataFields := map[string]interface{}{}

	if u.FirstName != nil {
		// use snake case to merge data with shopify
		dataFields["first_name"] = *u.FirstName
	}

	if u.LastName != nil {
		// use snake case to merge data with shopify
		dataFields["last_name"] = *u.LastName
	}

	if u.Zipcode != nil {
		dataFields["zipcode"] = *u.Zipcode
	}

	if u.City != nil {
		dataFields["city"] = *u.City
	}

	if u.State != nil {
		dataFields["state"] = *u.State
	}

	if u.Profile != nil {
		var riskProfile []*struct {
			RiskName string `json:"name"`
			Level    int    `json:"level"`
		}
		if err := json.Unmarshal([]byte(*u.Profile), &riskProfile); err != nil {
			tmplt := "unable to format risk profile(%s) for user(%d): %s\n"
			fmt.Printf(tmplt, *u.Profile, u.ID, err)
		} else {
			dataFields["riskProfile"] = riskProfile
		}
	}

	if u.Subscriptions != nil {
		var riskSubscriptions []string
		if err := json.Unmarshal([]byte(*u.Subscriptions), &riskSubscriptions); err != nil {
			fmt.Printf("unable to format risk subscriptions for user(%d): %s\n", u.ID, err)
		} else {
			dataFields["subscribedRisks"] = riskSubscriptions
		}
	}

	return dataFields
}
