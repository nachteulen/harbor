// TODO: deprecate the API dependencies and fetch entirely from DB
package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/aws/aws-lambda-go/events"
)

func getDocsAPIData(
	respBody *RespBody,
	wg *sync.WaitGroup,
	req *events.APIGatewayProxyRequest,
) {
	defer wg.Done()

	docsReq, _ := http.NewRequest("GET", documentsURL, nil)
	docsReq.Header.Add("Authorization", req.Headers["Authorization"])

	resp, err := retryClient.Do(docsReq)
	if err != nil {
		userID := req.RequestContext.Authorizer["userID"].(string)
		panic(fmt.Sprintf("documents request for user(%s) failed: %s", userID, err))
	}
	defer resp.Body.Close()

	var docsResp struct {
		Results []map[string]interface{} `json:"results"`
	}
	if err = json.NewDecoder(resp.Body).Decode(&docsResp); err != nil {
		userID := req.RequestContext.Authorizer["userID"].(string)
		panic(fmt.Sprintf("unable to parse documents request for user(%s): %s", userID, err))
	}

	if len(docsResp.Results) == 0 {
		respBody.Docs = []map[string]interface{}{}
	} else {
		respBody.Docs = docsResp.Results
	}
}

func getSafeLocationsAPIData(
	respBody *RespBody,
	wg *sync.WaitGroup,
	req *events.APIGatewayProxyRequest,
) {
	defer wg.Done()

	safeLocationsReq, _ := http.NewRequest("GET", safeLocationsURL, nil)
	safeLocationsReq.Header.Add("Authorization", req.Headers["Authorization"])

	resp, err := retryClient.Do(safeLocationsReq)
	if err != nil {
		userID := req.RequestContext.Authorizer["userID"].(string)
		panic(fmt.Sprintf("documents request for user(%s) failed: %s", userID, err))
	}
	defer resp.Body.Close()

	safeLocationsResp := []map[string]interface{}{}
	if err = json.NewDecoder(resp.Body).Decode(&safeLocationsResp); err != nil {
		userID := req.RequestContext.Authorizer["userID"].(string)
		panic(fmt.Sprintf("unable to parse safe locations request for user(%s): %s", userID, err))
	}

	// the client expects "ID", not "Id"
	for _, loc := range safeLocationsResp {
		catID, ok := loc["safeLocationCategoryId"]
		if ok {
			loc["safeLocationCategoryID"] = catID
		}
	}

	respBody.SafeLocations = safeLocationsResp
}
