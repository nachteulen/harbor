package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
)

const TOKENS_TABLE = "tokens"

var db *dynamodb.DynamoDB

type ErrorBody struct {
	ErrorMsg string `json:"error"`
}

type OtpVerificationRequest struct {
	Nonce string `json:"nonce"`
	OTP   string `json:"otp"`
}

type Token struct {
	Name  string `json:"tokenName"`
	Value string `json:"tokenValue,omitempty"`
	TTL   int64  `json:"ttl,omitempty"`
}

func (k *Token) valid() bool {
	if len(k.Value) == 0 {
		return false
	}
	ttl := time.Unix(k.TTL, 0)
	return time.Now().UTC().Before(ttl)
}

func makeResponse(status int, v bool) (*events.APIGatewayProxyResponse, error) {
	return &events.APIGatewayProxyResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       fmt.Sprintf(`{"success": %s}`, strconv.FormatBool(v)),
	}, nil
}

func handler(req events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	var o OtpVerificationRequest
	if err := json.Unmarshal([]byte(req.Body), &o); err != nil {
		fmt.Printf("invalid otp verification request: %s\n", err)
		return makeResponse(400, false)
	}

	if o.Nonce == "" || o.OTP == "" {
		fmt.Printf("nonce and otp required, received: %+v\n", o)
		return makeResponse(400, false)
	}

	key, _ := dynamodbattribute.MarshalMap(Token{Name: o.Nonce})
	result, err := db.GetItem(&dynamodb.GetItemInput{
		Key:       key,
		TableName: aws.String(TOKENS_TABLE),
	})
	if err != nil {
		fmt.Printf("error getting token(%s): %s\n", o.Nonce, err)
		return makeResponse(500, false)
	} else if result.Item == nil {
		fmt.Printf("token(%s) not found\n", o.Nonce)
		return makeResponse(404, false)
	}

	var t1 Token
	if err = dynamodbattribute.UnmarshalMap(result.Item, &t1); err != nil {
		fmt.Printf("error parsing token(%s): %s\n", o.Nonce, err)
		return makeResponse(500, false)
	} else if !t1.valid() {
		fmt.Printf("token(%+v) is invalid\n", t1)
		return makeResponse(404, false)
	}

	key, _ = dynamodbattribute.MarshalMap(Token{Name: t1.Value})
	result, err = db.GetItem(&dynamodb.GetItemInput{
		Key:       key,
		TableName: aws.String(TOKENS_TABLE),
	})
	if err != nil {
		fmt.Printf("error getting token(%s): %s\n", t1.Value, err)
		return makeResponse(500, false)
	} else if result.Item == nil {
		fmt.Printf("token(%s) not found\n", t1.Value)
		return makeResponse(404, false)
	}

	var t2 Token
	if err = dynamodbattribute.UnmarshalMap(result.Item, &t2); err != nil {
		fmt.Printf("error parsing token(%s): %s\n", t1.Value, err)
		return makeResponse(500, false)
	} else if !t2.valid() {
		fmt.Printf("token(%+v) is invalid\n", t2)
		return makeResponse(404, false)
	}

	if t2.Value != o.OTP {
		fmt.Printf("provided otp: %s did not match: %s\n", t2.Value, o.OTP)
		return makeResponse(404, false)
	}

	return makeResponse(200, true)
}

func init() {
	db = dynamodb.New(session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	})))
}

func main() {
	lambda.Start(handler)
}
