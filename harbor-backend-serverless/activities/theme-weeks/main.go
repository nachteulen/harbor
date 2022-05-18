package main

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var pgDB *sqlx.DB

type Activity struct {
	Name      string  `json:"name"`
	ID        int64   `json:"id"`
	LID       int64   `json:"levelID"`
	Readiness float32 `json:"readiness"`
}

type Theme struct {
	ID         int64       `json:"id"`
	Completed  bool        `json:"completed"`
	Name       string      `json:"name"`
	Activities []*Activity `json:"activities"`
}

type RowResult struct {
	ThemeID     int64   `db:"theme_id"`
	Theme       string  `db:"theme"`
	Ordering    int     `db:"ordering"`
	ID          int64   `db:"activity_id"`
	AName       string  `db:"name"`
	LevelID     int64   `db:"level_id"`
	Readiness   float32 `db:"readiness" json:"readiness"`
	DaysElapsed float64 `db:"days_elapsed"`
}

type RespBody struct {
	Current      *Theme   `json:"current"`
	NotCompleted []*Theme `json:"notCompleted"`
	Completed    []*Theme `json:"completed"`
	IsFirstCycle bool     `json:"isFirstCycle"`
}

func getWeekIdx(daysElapsed float64, numWeeks int) (int, bool) {
	if daysElapsed < 1 {
		return 0, true
	}

	isFirstCycle := int(daysElapsed) <= (numWeeks-1)*7
	idx := int(math.Ceil(daysElapsed/float64(7))) % (numWeeks)
	return idx, isFirstCycle
}

func parseThemes(idx int, results []*Theme, isFirstCycle bool) (*Theme, []*Theme, []*Theme) {
	var (
		current      *Theme
		completed    []*Theme
		notCompleted []*Theme
	)

	// if we're in the first cycle,
	// return the current week regardless of completion status
	if isFirstCycle {
		current = results[idx]
		if results[idx].Completed {
			completed = append(completed, current)
		} else {
			notCompleted = append(notCompleted, current)
		}
	} else if !results[idx].Completed {
		current = results[idx]
		notCompleted = append(notCompleted, current)
	} else {
		completed = append(completed, results[idx])
	}

	// otherwise start searching subsequent weeks,
	// until we've looped back around to our starting position
	next := idx + 1
	if next >= len(results) {
		next = 0
	}

	for next != idx && next < len(results) {
		if results[next] == nil {
			next = (next + 1) % len(results)
			continue
		}

		if results[next].Completed {
			completed = append(completed, results[next])
		} else {
			if current == nil {
				current = results[next]
			}
			notCompleted = append(notCompleted, results[next])
		}
		next = (next + 1) % len(results)
	}

	// else the user has completed all activities, so we'll pick one at random
	if current == nil {
		current = results[rand.Intn(len(results))]
	}

	return current, completed, notCompleted
}

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	userID := req.RequestContext.Authorizer["userID"].(string)
	oStr := req.RequestContext.Authorizer["allUserOwnershipsJSON"].(string)

	// admin can request info about a user
	onBehalfOf, ok := req.QueryStringParameters["userID"]
	if ok && len(onBehalfOf) != 0 && isAdmin(userID) {
		userID = onBehalfOf
		oStr = getOwnershipsStr(userID)
	}

	var ownerships []int64
	if err := json.Unmarshal([]byte(oStr), &ownerships); err != nil {
		panic(fmt.Sprintf("unable to parse ownerships for user(%s): %s", userID, err))
	}

	query, args, _ := sqlx.In(query, ownerships)
	query = pgDB.Rebind(query)

	var results []*RowResult
	err := pgDB.Select(&results, fmt.Sprintf(query, userID, userID), args...)
	if err != nil {
		panic(fmt.Errorf("error getting weekly theme for user(%+v): %s", userID, err))
	}

	if len(results) == 0 {
		return &events.APIGatewayProxyResponse{StatusCode: 404}, nil
	}

	var (
		groupedResults             []*Theme
		groupedResultsIdx          = -1
		currentTheme               string
		currentThemeCompletedCount int
	)
	for _, r := range results {
		if r.Theme != currentTheme {
			currentTheme = r.Theme
			currentThemeCompletedCount = 0
			groupedResultsIdx = groupedResultsIdx + 1
			groupedResults = append(groupedResults, &Theme{
				ID:   r.ThemeID,
				Name: r.Theme,
			})
		}
		t := groupedResults[groupedResultsIdx]
		t.Activities = append(t.Activities, &Activity{
			Name:      r.AName,
			ID:        r.ID,
			LID:       r.LevelID,
			Readiness: r.Readiness,
		})
		if r.Readiness == 1 {
			currentThemeCompletedCount = currentThemeCompletedCount + 1
		}
		t.Completed = len(t.Activities) == currentThemeCompletedCount
	}

	weekIdx, isFirstCycle := getWeekIdx(results[0].DaysElapsed, len(groupedResults))
	current, completed, notCompleted := parseThemes(weekIdx, groupedResults, isFirstCycle)

	b, _ := json.Marshal(RespBody{
		Current:      current,
		NotCompleted: notCompleted,
		Completed:    completed,
		IsFirstCycle: isFirstCycle,
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
}

func main() {
	lambda.Start(handler)
}
