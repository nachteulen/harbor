package main

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/aws/aws-lambda-go/events"
)

func getDBData(
	respBody *RespBody,
	wg *sync.WaitGroup,
	req *events.APIGatewayProxyRequest,
) {
	defer wg.Done()
	userID := req.RequestContext.Authorizer["userID"].(string)

	// initialize these, the client does not expect `null`
	respBody.Contacts = []map[string]interface{}{}
	respBody.Members = []map[string]interface{}{}
	respBody.LocalAuthorities = []map[string]interface{}{}

	var results []*struct {
		Meta string `db:"meta"`
		Type string `db:"row_type"`
	}
	if err := pgDB.Select(&results, query, userID); err != nil {
		panic(fmt.Sprintf("unable to get household data for user(%s): %s", userID, err))
	}

	for _, r := range results {
		if r.Type == "local_authorities" {
			var datum []map[string]interface{}
			json.Unmarshal([]byte(r.Meta), &datum)
			respBody.LocalAuthorities = datum
			continue
		}

		var datum map[string]interface{}
		json.Unmarshal([]byte(r.Meta), &datum)

		switch rType := r.Type; rType {
		case "contact":
			respBody.Contacts = append(respBody.Contacts, datum)
		case "member":
			user, ok := datum["user"]
			if ok {
				v, ok := user.(map[string]interface{})
				if ok && v["id"] == nil {
					datum["user"] = nil
				}
			}
			respBody.Members = append(respBody.Members, datum)
		}
	}
}
