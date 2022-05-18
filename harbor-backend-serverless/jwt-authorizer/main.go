package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sns"
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis/v8"
)

var (
	snsSvc *sns.SNS
	rDB    *redis.Client
	PEPPER = os.Getenv("PEPPER")
)

type MyCustomClaims struct {
	RefreshTokenID int64 `json:"refreshTokenId,omitempty"`
	UserID         int64 `json:"userId,omitempty"`
	jwt.StandardClaims
}

func generatePolicy(
	resource,
	accessToken string,
	claims *MyCustomClaims,
) *events.APIGatewayCustomAuthorizerResponse {
	authResponse := events.APIGatewayCustomAuthorizerResponse{
		PrincipalID: "apigateway.amazonaws.com",
	}
	authResponse.PolicyDocument = events.APIGatewayCustomAuthorizerPolicy{
		Version: "2012-10-17",
		Statement: []events.IAMPolicyStatement{
			{
				Action:   []string{"execute-api:Invoke"},
				Effect:   "Allow",
				Resource: []string{resource},
			},
		},
	}

	if claims == nil {
		return &authResponse
	}

	b, _ := json.Marshal(getOwnerships(claims.UserID))

	authResponse.Context = map[string]interface{}{
		"userID":                claims.UserID,
		"allUserOwnershipsJSON": string(b),
		"accessToken":           accessToken,
	}

	return &authResponse
}

func handler(request events.APIGatewayCustomAuthorizerRequest) (
	*events.APIGatewayCustomAuthorizerResponse, error,
) {
	accessToken := strings.Replace(request.AuthorizationToken, "Bearer ", "", 1)

	token, err := jwt.ParseWithClaims(
		accessToken,
		&MyCustomClaims{},
		func(token *jwt.Token) (interface{}, error) {
			return []byte(PEPPER), nil
		},
	)
	if err != nil {
		fmt.Printf("error parsing token(%s): %s\n", request.AuthorizationToken, err)
		return nil, errors.New("Unauthorized")
	}

	if claims, ok := token.Claims.(*MyCustomClaims); ok && token.Valid {
		return generatePolicy(request.MethodArn, accessToken, claims), nil
	}

	fmt.Printf("invalid token(%s)\n", request.AuthorizationToken)
	return nil, errors.New("Unauthorized")
}

func init() {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	}))
	snsSvc = sns.New(sess)

	opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		tmplt := "JWTAuth unable to establish Redis connection: %s"
		maybeAlertTom("redisConnError", fmt.Sprintf(tmplt, err))
		return
	}
	rDB = redis.NewClient(opt)
}

func main() {
	lambda.Start(handler)
}
