package main

import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-lambda-go/events"
)

const legacyQuery = `
with answer as (
	select id, household_id, input_id
	from form_input_answers
	where id = $1 and household_id = $2
), input as (
	select type, data_type
	from form_inputs
	where id = (select input_id from answer)
), all_options as (
	select meta -> 'options'
	from form_inputs
	where id = (select input_id from answer)
), filtered_options as (
	select value ->> 'value' as value, value ->> 'points' as points
	from jsonb_array_elements((select * from all_options))
	where value != 'null'
), multiselect_target as (
	select points::integer
	from filtered_options
	where value = $3
), multiselect_points as (
	select case
	when exists(select points from multiselect_target)
		then (select points from multiselect_target)
	else 0 end as points
), user_answer as (
	select $4 as value
), points as (
	select case
	when (select type from input) = 'multiselect'
		then (select points from multiselect_points)
	when (select type from input) = 'capture' and length((select value from user_answer)) != 0
		then 1
	else
		0
	end as points
), validated as (
	select case
		when (select data_type from input) = 'integer'
			then (select value from user_answer)::integer::text
		when (select data_type from input) = 'date' and length((select value from user_answer)) != 0
			then (select value from user_answer)::date::text
		else
			(select value from user_answer)
		end as value
)
update form_input_answers
set
	value = (select value from validated),
	points = (select points from points),
	user_id = $5,
	meta = $6
where id = (select id from answer)
and household_id = (select household_id from answer)`

func legacyUpdate(
	answerID,
	userID string,
	hhID int64,
	reqBody *ReqBody,
) (
	*events.APIGatewayProxyResponse, error,
) {
	args := []interface{}{
		answerID,
		hhID,
		reqBody.Answer,
		reqBody.Answer,
		userID,
	}

	if reqBody.AnswerMeta != nil {
		// TODO: add validation to prevent future parsing errors
		b, _ := json.Marshal(*reqBody.AnswerMeta)
		args = append(args, b)
	} else {
		args = append(args, nil)
	}

	_, err := pgDB.Exec(
		legacyQuery,
		args...,
	)
	if err != nil {
		tmplt := "unable to update answer(%s) for user(%s): %s"
		panic(fmt.Errorf(tmplt, answerID, userID, err))
	}

	return &events.APIGatewayProxyResponse{StatusCode: 204}, nil
}
