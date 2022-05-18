package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/go-redis/redis/v8"
	"github.com/helloharbor/golang-lib/form-inputs/meta"
	hhLib "github.com/helloharbor/harbor-backend-serverless/households/lib"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	pgDB *sqlx.DB
	rDB  *redis.Client
)

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	riskID := req.PathParameters["id"]
	userID := req.RequestContext.Authorizer["userID"].(string)
	hhID := hhLib.GetCurrentHouseholdID(userID, rDB, pgDB)
	oStr := req.RequestContext.Authorizer["allUserOwnershipsJSON"].(string)

	maxVersion := "1"
	v, ok := req.QueryStringParameters["maxPlanBuilderVersion"]
	if ok {
		maxVersion = v
	}

	var result struct {
		ID               int64   `db:"id"`
		Name             string  `db:"name"`
		Disclaimer       string  `db:"disclaimer"`
		Description      string  `db:"description"`
		IsPriority       bool    `db:"is_priority"`
		IsSubscribed     bool    `db:"is_subscribed"`
		LevelID          *int    `db:"level_id"`
		LevelText        *string `db:"level_text"`
		LevelColor       *string `db:"level_color"`
		PID              int64   `db:"plan_id"`
		PName            string  `db:"plan_name"`
		FormsJSON        string  `db:"forms_json"`
		InputsJSON       *string `db:"inputs_json"`
		InputAnswersJSON *string `db:"input_answers_json"`
		GuideJSON        *string `db:"guide_json"`
		RiskPlanProgress float64 `db:"risk_plan_progress"`
		ThemesJSON       *string `db:"themes_json"`
	}
	err := pgDB.Get(&result, query, userID, oStr, riskID, maxVersion, hhID, hhID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &events.APIGatewayProxyResponse{StatusCode: 404}, nil
		}
		panic(fmt.Errorf("error getting risk(%s) for user(%s): %s", riskID, userID, err))
	}

	answers := map[string]struct {
		ID    int64  `json:"answerID"`
		Value string `json:"answerValue"`
	}{}
	if result.InputAnswersJSON != nil {
		if err := json.Unmarshal([]byte(*result.InputAnswersJSON), &answers); err != nil {
			tmplt := "unable to parse form input answers(%s) for user(%s): %s"
			panic(fmt.Errorf(tmplt, *result.InputAnswersJSON, userID, err))
		}
	}

	inputs := map[string]*struct {
		ID       int64                  `json:"id"`
		Type     string                 `json:"type"`
		DataType string                 `json:"dataType"`
		AnswerID *int64                 `json:"answerID"`
		Meta     map[string]interface{} `json:"meta"`
		IsGlobal bool                   `json:"isGlobal"`
	}{}
	if result.InputsJSON != nil {
		if err := json.Unmarshal([]byte(*result.InputsJSON), &inputs); err != nil {
			tmplt := "unable to parse form inputs(%s) for user(%s): %s"
			panic(fmt.Errorf(tmplt, *result.InputsJSON, userID, err))
		}
	}

	forms := []*struct {
		ID       int64                    `json:"id"`
		Text     string                   `json:"text"`
		InputIDs []int64                  `json:"inputIDs"`
		Inputs   []map[string]interface{} `json:"inputs"`
	}{}
	if err := json.Unmarshal([]byte(result.FormsJSON), &forms); err != nil {
		tmplt := "unable to parse forms(%s) for user(%s): %s"
		panic(fmt.Errorf(tmplt, result.FormsJSON, userID, err))
	}

	for _, f := range forms {
		for _, inputID := range f.InputIDs {
			input, ok := inputs[fmt.Sprintf("%d", inputID)]
			if !ok {
				fmt.Printf("no input(%d) found for form(%d)\n", inputID, f.ID)
				continue
			}

			iMap := map[string]interface{}{
				"id":       inputID,
				"answerID": nil,
				"type":     input.Type,
				"dataType": input.DataType,
			}

			var key string
			if input.IsGlobal {
				key = fmt.Sprintf(":%d", inputID)
			} else {
				key = fmt.Sprintf("%d:%d", f.ID, inputID)
			}

			answer, ok := answers[key]
			if ok {
				// "monkey patch" value on
				input.Meta["answerValue"] = answer.Value
				iMap["answerID"] = answer.ID
			}

			meta.SetMeta(input.Meta, iMap, input.Type, input.DataType, inputID)
			f.Inputs = append(f.Inputs, iMap)
			// "monkey patch" value off
			// TODO: this is a terrible hack, `SetMeta` should not mutate data...
			input.Meta["answerValue"] = nil
		}
	}

	guide := struct {
		EID      int64                    `json:"eventID"`
		EName    string                   `json:"eventName"`
		Contents []map[string]interface{} `json:"steps"`
	}{}
	if result.GuideJSON != nil {
		if err := json.Unmarshal([]byte(*result.GuideJSON), &guide); err != nil {
			tmplt := "unable to parse guide(%s) for user(%s): %s\n"
			fmt.Printf(tmplt, *result.GuideJSON, userID, err)
		}
	}

	themes := []*struct {
		ID       int64   `json:"id"`
		Name     string  `json:"name"`
		Progress float64 `json:"progress"`
	}{}
	if result.ThemesJSON != nil {
		if err := json.Unmarshal([]byte(*result.ThemesJSON), &themes); err != nil {
			tmplt := "unable to parse themes(%s) for user(%s): %s\n"
			fmt.Printf(tmplt, *result.ThemesJSON, userID, err)
		}
	}

	var relatedThemesProgress float64
	for _, t := range themes {
		portion := (float64(50) / float64(len(themes)))
		relatedThemesProgress += t.Progress * (portion / 100)
	}
	readiness := (result.RiskPlanProgress * 0.5) + relatedThemesProgress

	name := result.Name
	if name[len(name)-1:] == "s" {
		name = name[0 : len(name)-1]
		if name[len(name)-2:] == "oe" {
			name = name[0 : len(name)-1]
		}
	}
	inventories := []map[string]interface{}{
		map[string]interface{}{
			"id":   nil,
			"name": name + " supplies",
		},
	}

	b, _ := json.Marshal(map[string]interface{}{
		"id":           result.ID,
		"name":         result.Name,
		"disclaimer":   result.Disclaimer,
		"description":  result.Description,
		"isPriority":   result.IsPriority,
		"isSubscribed": result.IsSubscribed,
		"levelID":      result.LevelID,
		"levelText":    result.LevelText,
		"levelColor":   result.LevelColor,
		"readiness":    readiness,
		"plan": map[string]interface{}{
			"id":    result.PID,
			"name":  result.PName,
			"forms": forms,
		},
		"guide":               guide,
		"themes":              themes,
		"inventoryCategories": inventories,
	})

	return &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Body:       string(b),
		Headers:    map[string]string{"Content-Type": "application/json"},
	}, nil
}

func init() {
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
