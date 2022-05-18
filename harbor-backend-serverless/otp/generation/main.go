package main

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strconv"
	_ "strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	twilio "github.com/kevinburke/twilio-go"
	uuid "github.com/nu7hatch/gouuid"
)

const TOKENS_TABLE = "tokens"

var (
	db          *dynamodb.DynamoDB
	twilioSID   = os.Getenv("TWILIO_SID")
	twilioToken = os.Getenv("TWILIO_TOKEN")
)

type ErrorBody struct {
	ErrorMsg string `json:"error"`
}

type OtpSMSGenerationRequest struct {
	PhoneNumber string `json:"phone_number"`
}

type Token struct {
	Name  string `json:"tokenName"`
	Value string `json:"tokenValue"`
	TTL   int64  `json:"ttl"`
}

func errorResponse(status int, msg string) (*events.APIGatewayProxyResponse, error) {
	body, _ := json.Marshal(ErrorBody{ErrorMsg: msg})

	return &events.APIGatewayProxyResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(body),
	}, nil
}

func generateOTP() (string, error) {
	nBig, err := rand.Int(rand.Reader, big.NewInt(899999))
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(nBig.Int64()+100000, 10), nil
}

func handler(req events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	var o OtpSMSGenerationRequest
	if err := json.Unmarshal([]byte(req.Body), &o); err != nil {
		fmt.Printf("invalid otp generation request: %s", err)
		return errorResponse(400, "Bad Request")
	}

	otp, err := generateOTP()
	if err != nil {
		fmt.Printf("unable to generate OTP: %s", err)
		return errorResponse(500, "Internal Server Error")
	}

	var phoneNumber string
	for _, c := range o.PhoneNumber {
		sC := string(c)
		_, err := strconv.Atoi(sC)
		if err != nil {
			continue
		} else if len(phoneNumber) == 0 && sC == "1" {
			continue
		}
		phoneNumber = phoneNumber + sC
	}

	if len(phoneNumber) != 10 {
		msg := fmt.Sprintf("invalid phone number: %s", o.PhoneNumber)
		return errorResponse(400, msg)
	}

	i, _ := dynamodbattribute.MarshalMap(Token{
		Name:  phoneNumber,
		Value: otp,
		TTL:   time.Now().Add(5 * time.Minute).Unix(),
	})
	_, err = db.PutItem(&dynamodb.PutItemInput{
		Item:      i,
		TableName: aws.String(TOKENS_TABLE),
	})
	if err != nil {
		fmt.Printf("could not set otp: %s", err)
		return errorResponse(500, "Internal Server Error")
	}

	u4, err := uuid.NewV4()
	nonce := u4.String()
	if err != nil {
		fmt.Printf("unable to generate nonce: %s", err)
		return errorResponse(500, "Internal Server Error")
	}

	i, _ = dynamodbattribute.MarshalMap(Token{
		Name:  nonce,
		Value: phoneNumber,
		TTL:   time.Now().Add(5 * time.Minute).Unix(),
	})
	_, err = db.PutItem(&dynamodb.PutItemInput{
		Item:      i,
		TableName: aws.String(TOKENS_TABLE),
	})
	if err != nil {
		fmt.Printf("could not store nonce: %s", err)
		return errorResponse(500, "Internal Server Error")
	}

	client := twilio.NewClient(twilioSID, twilioToken, nil)
	_, err = client.Messages.SendMessage(
		"+17755427267",
		"+1"+phoneNumber,
		fmt.Sprintf("Your harbor code is %s", otp),
		nil,
	)
	if err != nil {
		panic(fmt.Errorf("error sending SMS(%s): %s", phoneNumber, err))
	}

	return &events.APIGatewayProxyResponse{
		StatusCode: 201,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       fmt.Sprintf(`{"nonce": "%s"}`, nonce),
	}, nil
}

func init() {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	}))
	db = dynamodb.New(sess)
}

func main() {
	lambda.Start(handler)
}
