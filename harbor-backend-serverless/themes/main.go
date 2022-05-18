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

type Snack struct {
	Type  string                 `json:"type"`
	Title string                 `json:"title"`
	Body  string                 `json:"body"`
	Meta  map[string]interface{} `json:"meta"`
}

func handler(req events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	userID := req.RequestContext.Authorizer["userID"].(string)
	hhID := hhLib.GetCurrentHouseholdID(userID, rDB, pgDB)
	themeID := req.PathParameters["id"]
	oStr := req.RequestContext.Authorizer["allUserOwnershipsJSON"].(string)

	maxVersion := "1"
	v, ok := req.QueryStringParameters["maxPlanBuilderVersion"]
	if ok {
		maxVersion = v
	}

	var result struct {
		Readiness        float64 `db:"readiness"`
		Theme            string  `db:"theme"`
		Description      string  `db:"description"`
		RelatedRiskIDs   *string `db:"related_risk_ids"`
		ID               int64   `db:"id"`
		Name             string  `db:"name"`
		FormsJSON        string  `db:"forms_json"`
		InputsJSON       *string `db:"inputs_json"`
		SnacksJSON       *string `db:"snacks_json"`
		InputAnswersJSON *string `db:"input_answers_json"`
		ActivitiesJSON   *string `db:"activities_json"`
		InventoriesJSON  *string `db:"inventories_json"`
	}
	err := pgDB.Get(&result, query, themeID, maxVersion, hhID, userID, oStr, themeID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &events.APIGatewayProxyResponse{StatusCode: 404}, nil
		}
		panic(fmt.Errorf("error getting theme(%s) for user(%s): %s", themeID, userID, err))
	}

	answers := map[string]struct {
		ID    int64                  `json:"answerID"`
		Value string                 `json:"answerValue"`
		Meta  map[string]interface{} `json:"answerMeta"`
	}{}
	if result.InputAnswersJSON != nil {
		if err := json.Unmarshal([]byte(*result.InputAnswersJSON), &answers); err != nil {
			tmplt := "unable to parse form input answers(%s) for user(%s): %s"
			panic(fmt.Errorf(tmplt, *result.InputAnswersJSON, userID, err))
		}
	}

	inputs := map[string]*struct {
		Type     string                 `json:"type"`
		DataType string                 `json:"dataType"`
		Meta     map[string]interface{} `json:"meta"`
		IsGlobal bool                   `json:"isGlobal"`
	}{}
	if result.InputsJSON != nil {
		if err := json.Unmarshal([]byte(*result.InputsJSON), &inputs); err != nil {
			tmplt := "unable to parse form inputs(%s) for user(%s): %s"
			panic(fmt.Errorf(tmplt, *result.InputsJSON, userID, err))
		}
	}

	snacks := map[string]*Snack{}
	if result.SnacksJSON != nil {
		if err := json.Unmarshal([]byte(*result.SnacksJSON), &snacks); err != nil {
			tmplt := "unable to parse snacks(%s) for user(%s): %s"
			panic(fmt.Errorf(tmplt, *result.SnacksJSON, userID, err))
		}
	}

	forms := []*struct {
		ID       int64                    `json:"id"`
		Text     string                   `json:"text"`
		InputIDs []int64                  `json:"inputIDs"`
		Inputs   []map[string]interface{} `json:"inputs"`
		SnackIDs []int64                  `json:"snackIDs"`
		Snacks   []*Snack                 `json:"snacks"`
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
				// "monkey patch" `value` and `meta`
				input.Meta["answerValue"] = answer.Value
				input.Meta["answerMeta"] = answer.Meta
				iMap["answerID"] = answer.ID
			}

			meta.SetMeta(input.Meta, iMap, input.Type, input.DataType, inputID)
			f.Inputs = append(f.Inputs, iMap)
			// "monkey patch" value off
			// TODO: this is a terrible hack, `SetMeta` should not mutate data...
			input.Meta["answerValue"] = nil
		}

		for _, snackID := range f.SnackIDs {
			snack, ok := snacks[fmt.Sprintf("%d", snackID)]
			if !ok {
				fmt.Printf("no snack(%d) found for form(%d)\n", snackID, f.ID)
				continue
			}
			f.Snacks = append(f.Snacks, snack)
		}
	}

	activities := []*struct {
		ID        int64   `json:"activityID"`
		LID       int     `json:"levelID"`
		Name      string  `json:"name"`
		Readiness float64 `json:"readiness"`
	}{}
	if result.ActivitiesJSON == nil {
		fmt.Printf("got null activities for user(%s)\n", userID)
	} else {
		if err := json.Unmarshal([]byte(*result.ActivitiesJSON), &activities); err != nil {
			tmplt := "unable to parse activities(%s) for user(%s): %s"
			panic(fmt.Errorf(tmplt, *result.ActivitiesJSON, userID, err))
		}
	}

	relatedRiskIDs := []int64{}
	if result.RelatedRiskIDs != nil {
		if err := json.Unmarshal([]byte(*result.RelatedRiskIDs), &relatedRiskIDs); err != nil {
			tmplt := "unable to parse relatedRiskIDs(%s) for user(%s): %s\n"
			fmt.Printf(tmplt, *result.RelatedRiskIDs, userID, err)
		}
	}

	inventories := []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}{}
	if result.InventoriesJSON != nil {
		if err := json.Unmarshal([]byte(*result.InventoriesJSON), &inventories); err != nil {
			tmplt := "unable to parse inventories(%s) for user(%s): %s\n"
			fmt.Printf(tmplt, *result.InventoriesJSON, userID, err)
		}
	}

	b, _ := json.Marshal(map[string]interface{}{
		"theme":       result.Theme,
		"description": result.Description,
		"readiness":   result.Readiness,
		"plan": map[string]interface{}{
			"id":    result.ID,
			"name":  result.Name,
			"forms": forms,
		},
		"activities":          activities,
		"relatedRiskIDs":      relatedRiskIDs,
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
