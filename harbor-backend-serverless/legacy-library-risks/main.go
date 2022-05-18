package main

import (
	"fmt"
	"os"
	"sort"

	hh "github.com/helloharbor/golang-lib/hazardhub"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	db   *sqlx.DB
	auth = os.Getenv("HAZARD_HUB_AUTH")
	url  = os.Getenv("HAZARD_HUB_URL")
)

type Request struct {
	UserID       int64    `json:"user_id"`
	OwnershipIDs [2]int64 `json:"ownership_ids"`
}

type Risks struct {
	ID             int64    `db:"id" json:"id"`
	Name           string   `db:"name" json:"name"`
	Disclaimer     string   `db:"disclaimer" json:"disclaimer"`
	ListImagePath  string   `db:"list_image_path" json:"list_image_path"`
	Icon           string   `db:"icon_image_path" json:"icon_image_path"`
	IsPriority     bool     `db:"is_priority" json:"is_priority"`
	RiskLevel      int      `json:"risk_level,omitempty"`
	RiskLevelText  string   `json:"risk_level_text,omitempty"`
	RiskLevelColor string   `json:"risk_level_color,omitempty"`
	Readiness      float64  `db:"readiness" json:"readiness"`
	IsSubscribed   bool     `db:"is_subscribed" json:"is_subscribed"`
	Zipcode        *string  `db:"zipcode" json:",omitempty"`
	State          *string  `db:"state_abbr" json:",omitempty"`
	Latitude       *float64 `db:"latitude" json:",omitempty"`
	Longitude      *float64 `db:"longitude" json:",omitempty"`
}

type ByRisk []*Risks

func (r ByRisk) Len() int {
	return len(r)
}

func (r ByRisk) Less(i, j int) bool {
	if r[i].RiskLevel > r[j].RiskLevel {
		return true
	}
	return r[i].Name < r[j].Name
}

func (r ByRisk) Swap(i, j int) {
	r[i], r[j] = r[j], r[i]
}

// The Node app calls this Lambda function directly, which is bad practice and
// makes monitoring difficult. We'd like to deprecate the Node endpoint handler
// along with this function.
func handler(req Request) ([]*Risks, error) {
	var results []*Risks
	err := db.Select(
		&results,
		query,
		req.UserID,
		req.OwnershipIDs[0],
		req.OwnershipIDs[1],
		req.OwnershipIDs[0],
		req.OwnershipIDs[1],
	)
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		fmt.Printf("no results found for user(%d)\n", req.UserID)
		return results, nil
	}

	if results[0].Latitude != nil && results[0].Longitude != nil {
		lat := *results[0].Latitude
		lng := *results[0].Longitude
		profile, err := hh.GetGeoRisks(auth, url, lat, lng)
		if err != nil {
			tmplt := "unable to get geo(%f,%f) risks for user(%d): %s"
			fmt.Printf(tmplt, lat, lng, req.UserID, err)
		} else {
			fmt.Println("got geo risks")
			return formatResponse(profile, results), nil
		}
	}

	if results[0].Zipcode != nil && results[0].State != nil {
		zip := *results[0].Zipcode
		state := *results[0].State
		profile, err := hh.GetZipRisks(auth, url, zip, &state)
		if err != nil {
			tmplt := "unable to get zip(%s,%s) risks for user(%d): %s"
			fmt.Printf(tmplt, state, zip, req.UserID, err)
		} else {
			fmt.Println("got zip risks")
			return formatResponse(profile, results), nil
		}
	}

	return results, nil
}

func init() {
	d, err := sqlx.Connect("postgres", os.Getenv("DB_CONN"))
	if err != nil {
		panic(err)
	}
	db = d
}

func main() {
	lambda.Start(handler)
}

func formatResponse(rP map[int]*hh.RiskProfile, risks []*Risks) []*Risks {
	var subscribedRisks []*Risks
	var unsubscribedRisks []*Risks

	for _, r := range risks {
		// remove the location data that we fetched earlier to avoid a
		// secondary db hit, the client does not need or expect this data.
		r.Zipcode = nil
		r.Latitude = nil
		r.Longitude = nil
		r.State = nil

		p, ok := rP[int(r.ID)]
		if !ok {
			fmt.Printf("no profile found for id(%d)\n", r.ID)
			p = &hh.RiskProfile{}
		}
		r.RiskLevel = p.Level
		r.RiskLevelColor = p.LevelColor
		r.RiskLevelText = p.LevelText

		if r.IsSubscribed {
			subscribedRisks = append(subscribedRisks, r)
		} else {
			unsubscribedRisks = append(unsubscribedRisks, r)
		}
	}

	sort.Sort(ByRisk(subscribedRisks))
	sort.Sort(ByRisk(unsubscribedRisks))

	// ensure subscribed risks appear first in the response
	return append(subscribedRisks, unsubscribedRisks...)
}
