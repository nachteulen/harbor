package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var connStr = fmt.Sprintf(
	"user=%s password=%s host=%s port=%s dbname=%s sslmode=%s",
	os.Getenv("DB_USER"),
	os.Getenv("DB_PASSWORD"),
	os.Getenv("DB_HOST"),
	os.Getenv("DB_PORT"),
	os.Getenv("DB_NAME"),
	os.Getenv("SSL_MODE"),
)

type RiskRow struct {
	ID                int64  `db:"id" json:"id"`
	Name              string `db:"name" json:"name"`
	Disclaimer        string `db:"disclaimer" json:"disclaimer"`
	ListImagePath     string `db:"list_image_path" json:"list_image_path"`
	IsPriority        bool   `db:"is_priority" json:"is_priority"`
	IsDefaultSelected bool   `db:"is_default_selected" json:"is_default_selected"`
	RiskLevel         int    `db:"risk_level" json:"risk_level,omitempty"`
	RiskLevelColor    string `db:"risk_level_text" json:"risk_level_text,omitempty"`
	RiskLevelText     string `db:"risk_level_color" json:"risk_level_color,omitempty"`
	// don't even select this from the DB, just let it default to 0
	Readiness int `json:"readiness"`
}

func handler(ctx context.Context, gRR events.APIGatewayProxyRequest) (
	*events.APIGatewayProxyResponse, error,
) {
	if gRR.Path == "/risks/onboarding-ping" {
		return &events.APIGatewayProxyResponse{StatusCode: 200}, nil
	}

	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "unable to establish DB connection",
		}, err
	}
	defer db.Close()

	query := `
with location as (
    select
        a.zipcode,
        a.latitude,
        a.longitude
    from
        users u
    join
        addresses a on u.address_id = a.id
    where
        u.id = $1
), risk_profiles as (
    select
        event_id,
        risk_level,
        rank() over(order by latitude, longitude)
    from
        location_risk_profiles lrp
    where (
        lrp.zipcode = (select zipcode from location)
        and lrp.latitude is null
        and lrp.longitude is null
    ) or (
        lrp.zipcode = (select zipcode from location)
        and lrp.latitude = (select latitude from location)
        and lrp.longitude = (select longitude from location)
    )
)
select
    e.id,
    e.name,
    e.disclaimer,
    f.path as list_image_path,
    case
        when e.is_priority or rp.risk_level >= 3 then true
        else false end as is_priority,
    case
        when e.is_priority or rp.risk_level >= 3 then true
        else false end as is_default_selected,
    coalesce(rp.risk_level, 0) as risk_level,
    coalesce(rl.attrs ->> 'text', '') as risk_level_text,
    coalesce(rl.attrs ->> 'color', '') as risk_level_color
from
    events e
left join
    files f on f.uuid = e.event_list_image_uuid
left join
    risk_profiles rp on rp.event_id = e.id and rp.rank = 1
left join
    risk_levels rl on rp.risk_level = rl.level
where
    e.enabled = true
order by
    is_priority desc,
    rp.risk_level desc,
    name;
`
	uID, ok := gRR.RequestContext.Authorizer["userID"]
	if !ok {
		return &events.APIGatewayProxyResponse{
			StatusCode: 400,
			Body:       "missing UserId from request context",
		}, errors.New("missing UserId from request context")
	}

	results := []*RiskRow{}
	err = db.SelectContext(ctx, &results, query, uID)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "DB error fetching results",
		}, err
	}

	b, err := json.Marshal(results)
	if err != nil {
		return &events.APIGatewayProxyResponse{
			StatusCode: 500,
			Body:       "unable to Marshal results",
		}, err
	}

	return &events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(b),
	}, nil
}

func main() {
	lambda.Start(handler)
}
