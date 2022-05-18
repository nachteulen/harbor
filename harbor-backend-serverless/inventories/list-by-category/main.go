package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
)

var (
	pgDB  *sqlx.DB
	s3SVC *s3.S3
)

type Category struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	UserStorage *struct {
		ID       int64   `json:"id"`
		Location string  `json:"location"`
		FileUUID *string `json:"fileUUID"`
		FileURL  *string `json:"fileURL"`
	} `json:"userStorage"`
}

func handler(req events.APIGatewayProxyRequest) (*events.APIGatewayProxyResponse, error) {
	categoryID := req.PathParameters["id"]
	oStr := req.RequestContext.Authorizer["allUserOwnershipsJSON"].(string)

	var result struct {
		Category string `db:"category_json"`
		Items    string `db:"inventory_items_json"`
	}
	err := pgDB.Get(&result, query, oStr, categoryID)
	if err != nil {
		if err == sql.ErrNoRows {
			return &events.APIGatewayProxyResponse{StatusCode: 404}, nil
		}
		panic(fmt.Errorf("error getting inventory items by category: %s", err))
	}

	var category Category
	if err := json.Unmarshal([]byte(result.Category), &category); err != nil {
		panic(fmt.Errorf("unable to parse category(%s): %s", result.Category, err))
	}

	if category.UserStorage != nil {
		catStore := *category.UserStorage

		if catStore.FileUUID != nil {
			if s3SVC == nil {
				// TODO: it should *not* be necessary to pass credentials here
				// ideally we can handle this via IAM, but I lost my patience...
				creds := credentials.NewStaticCredentials(
					os.Getenv("S3_KEY_ID"),
					os.Getenv("S3_ACCESS_KEY"),
					"",
				)
				s3SVC = s3.New(session.Must(session.NewSession(&aws.Config{
					Credentials: creds,
					Region:      aws.String(os.Getenv("AWS_REGION")),
				})))
			}
			key := fmt.Sprintf("private/%s.jpeg", *catStore.FileUUID)
			req, _ := s3SVC.GetObjectRequest(&s3.GetObjectInput{
				Bucket: aws.String(os.Getenv("BACKEND_S3_BUCKET")),
				Key:    aws.String(key),
			})
			url, err := req.Presign(time.Minute * 1)
			if err != nil {
				fmt.Printf("failed to presign url for key %s: %s\n", key, err)
			} else {
				category.UserStorage.FileURL = &url
			}
		}
	}

	items := []map[string]interface{}{}
	if err := json.Unmarshal([]byte(result.Items), &items); err != nil {
		panic(fmt.Errorf("unable to parse items(%s): %s", result.Items, err))
	}

	b, _ := json.Marshal(map[string]interface{}{
		"category": category,
		"items":    items,
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
}

func main() {
	lambda.Start(handler)
}
