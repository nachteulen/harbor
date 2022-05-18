package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	lambdaSVC "github.com/aws/aws-sdk-go/service/lambda"
	"github.com/go-redis/redis/v8"
	hhLib "github.com/helloharbor/harbor-backend-serverless/households/lib"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

const t = true

var (
	pgDB              *sqlx.DB
	rDB               *redis.Client
	basicSafetyThemes = map[int64]bool{
		1:  t, // Home
		13: t, // Learning
		15: t, // Planning
		31: t, // Communication
	}
	shelterInPlaceThemes = map[int64]bool{
		2: t, // Water
		4: t, // Health
		5: t, // Food
		7: t, // Power & Heat
	}
	gettingOutOfTownThemes = map[int64]bool{
		9:  t, // Go Bag
		21: t, // Places
		11: t, // Car
	}
)

type Risk struct {
	Name            string  `json:"name"`
	Level           *int    `json:"level"`
	LevelText       *string `json:"levelText"`
	LevelColor      *string `json:"levelColor"`
	Progress        float64 `json:"progress"`
	RelatedThemeIDs []int64 `json:"relatedThemeIDs"`
}

type Theme struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Progress float64 `json:"progress"`
}

type ScheduleItem struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

type Week struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Type     string  `json:"type"`
	Progress float64 `json:"progress"`
}

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	userID := req.RequestContext.Authorizer["userID"].(string)
	oStr := req.RequestContext.Authorizer["allUserOwnershipsJSON"].(string)

	// harbor admin can request info about a user
	onBehalfOf := req.QueryStringParameters["userID"]
	if onBehalfOf != "" && userID == "1" {
		userID = onBehalfOf
		oStr = getOwnershipsStr(userID)
	}

	hhID := hhLib.GetCurrentHouseholdID(userID, rDB, pgDB)

	maxVersion := "1"
	v, ok := req.QueryStringParameters["maxPlanBuilderVersion"]
	if ok {
		maxVersion = v
	}

	var result struct {
		WeekIdx       int     `db:"week_idx"`
		ScheduleJSON  string  `db:"schedule_json"`
		ThemesJSON    string  `db:"themes_json"`
		RisksJSON     string  `db:"risks_json"`
		RisksOrdering string  `db:"risks_ordering"`
		Readiness     float32 `db:"readiness"`
		Rank          string  `db:"rank"`
		FoundNull     bool    `db:"found_null_schedule"`
		IsFirstCycle  bool    `db:"is_first_cycle"`
	}
	err := pgDB.Get(&result, query, userID, oStr, maxVersion, hhID)
	if err != nil {
		panic(fmt.Errorf("error getting schedule for user(%s): %s", userID, err))
	}

	// the user's schedule was missing,
	// so we defaulted to theme ordering...
	if result.FoundNull {
		fmt.Printf("no schedule found for user(%s)\n", userID)

		lambdaClient := lambdaSVC.New(session.Must(session.NewSession(&aws.Config{
			Region: aws.String(os.Getenv("AWS_REGION")),
		})))

		payload, _ := json.Marshal(events.APIGatewayProxyRequest{
			Body: fmt.Sprintf(`{"userID": %s}`, userID),
		})

		// ...but we'll try to set their schedule asynchronously
		// so this doesn't happen again
		if _, err := lambdaClient.Invoke(&lambdaSVC.InvokeInput{
			Payload:        payload,
			FunctionName:   aws.String("WeeklyScheduleUpsert"),
			InvocationType: aws.String("Event"),
		}); err != nil {
			fmt.Printf("invocation failed for user(%s): %s\n", userID, err)
		}
	}

	var themes map[string]*Theme
	if err := json.Unmarshal([]byte(result.ThemesJSON), &themes); err != nil {
		tmplt := "unable to parse themes(%s) for user(%s): %s"
		panic(fmt.Errorf(tmplt, result.ThemesJSON, userID, err))
	}

	basicSafety := []*Theme{}
	shelterInPlace := []*Theme{}
	gettingOutOfTown := []*Theme{}

	for _, t := range themes {
		if basicSafetyThemes[t.ID] {
			basicSafety = append(basicSafety, t)
		} else if shelterInPlaceThemes[t.ID] {
			shelterInPlace = append(shelterInPlace, t)
		} else if gettingOutOfTownThemes[t.ID] {
			gettingOutOfTown = append(gettingOutOfTown, t)
		} else {
			fmt.Printf("no mapping for Theme(%d)\n", t.ID)
			// append anyways, should never happen, and can't hurt
			basicSafety = append(basicSafety, t)
		}
	}

	var risks map[string]*Risk
	if err := json.Unmarshal([]byte(result.RisksJSON), &risks); err != nil {
		tmplt := "unable to parse risks(%s) for user(%s): %s"
		panic(fmt.Errorf(tmplt, result.RisksJSON, userID, err))
	}

	for _, r := range risks {
		var relatedThemesProgress float64
		for _, tID := range r.RelatedThemeIDs {
			t, ok := themes[fmt.Sprintf("%d", tID)]
			if !ok {
				fmt.Printf("theme(%d) missing for %s", tID, r.Name)
				continue
			}
			portion := (float64(50) / float64(len(r.RelatedThemeIDs)))
			relatedThemesProgress += t.Progress * (portion / 100)
		}
		r.Progress = (r.Progress * 0.5) + relatedThemesProgress
	}

	var risksOrder []*struct {
		ID         int64 `json:"id"`
		Subscribed bool  `json:"subscribed"`
	}
	if err := json.Unmarshal([]byte(result.RisksOrdering), &risksOrder); err != nil {
		tmplt := "unable to parse risksOrder(%s) for user(%s): %s"
		panic(fmt.Errorf(tmplt, result.RisksOrdering, userID, err))
	}

	var totalReadiness float64
	subscribedRisks := []map[string]interface{}{}
	unsubscribedRisks := []map[string]interface{}{}

	for _, o := range risksOrder {
		r, ok := risks[fmt.Sprintf("%d", o.ID)]
		if !ok {
			fmt.Printf("risk(%d) not found for user(%s)\n", o.ID, userID)
			continue
		}

		totalReadiness += r.Progress

		risk := map[string]interface{}{
			"id":             o.ID,
			"name":           r.Name,
			"readiness":      r.Progress,
			"riskLevel":      r.Level,
			"riskLevelText":  r.LevelText,
			"riskLevelColor": r.LevelColor,
		}

		if o.Subscribed {
			subscribedRisks = append(subscribedRisks, risk)
		} else {
			unsubscribedRisks = append(unsubscribedRisks, risk)
		}
	}

	var schedule []*ScheduleItem
	if err := json.Unmarshal([]byte(result.ScheduleJSON), &schedule); err != nil {
		tmplt := "unable to parse schedule(%s) for user(%s): %s"
		panic(fmt.Errorf(tmplt, result.ScheduleJSON, userID, err))
	}

	orderedWeeklySchedule := sortWeeklySchedule(
		risks, themes, schedule, result.WeekIdx, userID,
	)

	b, _ := json.Marshal(map[string]interface{}{
		"weeklySchedule": orderedWeeklySchedule,
		"readinessSummary": map[string]interface{}{
			"rank":      result.Rank,
			"readiness": result.Readiness,
		},
		"riskSummary": map[string]interface{}{
			"risksCount":       len(subscribedRisks),
			"averageReadiness": totalReadiness / float64(len(subscribedRisks)),
		},
		"risks":             subscribedRisks,
		"unsubscribedRisks": unsubscribedRisks,
		"basicSafety":       basicSafety,
		"shelterInPlace":    shelterInPlace,
		"gettingOutOfTown":  gettingOutOfTown,
		"isFirstCycle":      result.IsFirstCycle,
	})

	return &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(b),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func init() {
	if os.Getenv("TESTING") == ("1") {
		return
	}

	d, err := sqlx.Connect("postgres", os.Getenv("DB_CONN"))
	if err != nil {
		panic(err)
	}
	pgDB = d

	opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		panic(err)
	}
	rDB = redis.NewClient(opt)
}

func main() {
	lambda.Start(handler)
}
