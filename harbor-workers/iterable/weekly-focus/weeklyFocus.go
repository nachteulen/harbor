package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type Resp struct {
	WeeklySchedule []*struct {
		Name     string  `json:"name"`
		Type     string  `json:"type"`
		Progress float64 `json:"progress"`
	} `json:"weeklySchedule"`
	IsFirstCycle bool `json:"isFirstCycle"`
}

type WeeklyFocus struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	IsCompleted  bool   `json:"isCompleted"`
	IsFirstCycle bool   `json:"isFirstCycle"`
}

func getWeeklyFocusForUser(id int64) (*WeeklyFocus, error) {
	url := fmt.Sprintf(harborAPI2BaseURL+"/today?userID=%d", id)
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	resp, err := retryClient.Do(req)
	if err != nil {
		panic(fmt.Sprintf("request for user(%d) failed: %s", id, err))
	}

	if resp.StatusCode == 401 {
		authenticateAdminUser()
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", accessToken))
		resp, err = retryClient.Do(req)
		if err != nil {
			panic(fmt.Sprintf("retry request for user(%d) failed: %s", id, err))
		}
	}

	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		b, _ := ioutil.ReadAll(resp.Body)
		tmplt := "%d get weekly focus response for user(%d): %s"
		panic(fmt.Sprintf(tmplt, resp.StatusCode, id, string(b)))
	}

	var r Resp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		panic(fmt.Sprintf("unable to decode response for user(%d): %s", id, err))
	}

	if len(r.WeeklySchedule) == 0 {
		return nil, fmt.Errorf("no weekly focus for user(%d)", id)
	}

	wF := &WeeklyFocus{
		Name:         r.WeeklySchedule[0].Name,
		Type:         r.WeeklySchedule[0].Type,
		IsFirstCycle: r.IsFirstCycle,
	}

	if r.WeeklySchedule[0].Progress == 1 {
		wF.IsCompleted = true
	}

	return wF, nil
}
