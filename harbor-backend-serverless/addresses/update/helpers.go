package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/mmcloughlin/geohash"
)

type LocalAuthority struct {
	Name    string  `json:"name"`
	Address string  `json:"address"`
	Lat     float64 `json:"lat"`
	Lng     float64 `json:"lng"`
	Type    string  `json:"type"`
}

type HazardResponse struct {
	LocalAuthorities []*LocalAuthority `json:"localAuthorities"`
	Profile          map[string]int    `json:"profile"`
}

func insertRiskProfile(
	userID string,
	idParam int64,
	hazardResp *HazardResponse,
) error {
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
		return fmt.Errorf("invalid profile(%d): %+v", idParam, hazardResp.Profile)
	}

	profileB, _ := json.Marshal(profile)
	args := []interface{}{idParam, profileB}

	if len(hazardResp.LocalAuthorities) == 0 {
		args = append(args, nil)
	} else {
		authoritiesB, _ := json.Marshal(hazardResp.LocalAuthorities)
		args = append(args, authoritiesB)
	}

	args = append(args, userID)

	_, err := pgDB.Exec(insertProfileQuery, args...)
	if err != nil {
		tmplt := "unable to insert/update user(%s) address with profile(%s): %s"
		return fmt.Errorf(tmplt, userID, string(profileB), err)
	}
	return nil
}

func doUpdate(
	url,
	userID string,
	idParam int64,
	urlParams map[string]string,
) error {
	var exists bool
	if err := pgDB.Get(&exists, selectQuery, idParam); err != nil {
		fmt.Printf("error checking profile(%d): %s\n", idParam, err)
	}

	if exists {
		if _, err := pgDB.Exec(updateAddressQuery, userID, idParam); err != nil {
			tmplt := "unable to update user(%s) address with profile(%d): %s"
			return fmt.Errorf(tmplt, userID, idParam, err)
		}
		return nil
	}

	req, _ := http.NewRequest("GET", url, nil)
	q := req.URL.Query()
	for k, v := range urlParams {
		q.Add(k, v)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := retryClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to get profile(%d): %s", idParam, err)
	}
	defer resp.Body.Close()

	var hazardResp HazardResponse
	if err = json.NewDecoder(resp.Body).Decode(&hazardResp); err != nil {
		b, _ := ioutil.ReadAll(resp.Body)
		tmplt := "unable to decode profile(%s) for %d: %s"
		return fmt.Errorf(tmplt, string(b), idParam, err)
	}

	if err = insertRiskProfile(userID, idParam, &hazardResp); err != nil {
		tmplt := "unable to insert profile(%+v) for %d: %s"
		return fmt.Errorf(tmplt, hazardResp.Profile, idParam, err)
	}
	return nil
}

func handleGeoUpdate(lat, lng float64, userID string) error {
	idParam := geohash.EncodeIntWithPrecision(lat, lng, 64)
	urlParams := map[string]string{
		"lat": fmt.Sprintf("%f", lat),
		"lng": fmt.Sprintf("%f", lng),
	}
	return doUpdate(harborRiskProfileURL+"/byCoordinates", userID, int64(idParam), urlParams)
}

func handleZipUpdate(state, zipcode string, userID string) error {
	idParam, err := strconv.Atoi(zipcode)
	if err != nil {
		return fmt.Errorf("invalid zipcode: %s", zipcode)
	}
	urlParams := map[string]string{
		"zipcode": zipcode,
		"state":   state,
	}
	return doUpdate(harborRiskProfileURL+"/byState", userID, int64(idParam), urlParams)
}
