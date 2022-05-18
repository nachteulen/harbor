package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/sns"
)

var errorConfig = map[string]bool{}

func maybeAlertTom(key, body string) {
	if !errorConfig[key] {
		snsSvc.Publish(&sns.PublishInput{
			Message:     aws.String(body),
			PhoneNumber: aws.String("+17037173325"),
		})
		errorConfig[key] = true
	}
}
