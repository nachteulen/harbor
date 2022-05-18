package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/go-retryablehttp"
)

const probSevereURL = "https://mrms.ncep.noaa.gov/data/ProbSevere/PROBSEVERE"

// sort the page by "Last Modified" desc for convenience
const indexURL = probSevereURL + "/?C=M;O=D"

var (
	ctx         = context.Background()
	redisConn   *redis.Client
	retryClient *http.Client
	uploader    *s3manager.Uploader
	s3bucket    = os.Getenv("BUCKET_NAME")
	// decrease redis traffic with ephemeral local cache
	localCache = map[string]bool{}
)

func shouldBreakForHref(href string) bool {
	if localCache[href] {
		return true
	}

	exists, err := redisConn.Exists(ctx, href).Result()
	if err != nil {
		fmt.Printf("unable to get redis(%s): %s\n", href, err)
	} else if exists == 1 {
		return true
	}

	url := probSevereURL + "/" + href
	resp, err := retryClient.Get(url)
	if err != nil {
		panic(fmt.Errorf("unable to get(%s): %s", url, err))
	}
	defer resp.Body.Close()

	pathParts := strings.Split(href, "_")
	if len(pathParts) < 4 {
		panic(fmt.Errorf("malformed href path parts: %s", href))
	}

	keyParts := strings.Split(pathParts[2]+pathParts[3], ".")
	if len(keyParts) < 1 {
		panic(fmt.Errorf("malformed href key parts: %s", href))
	}

	upParams := &s3manager.UploadInput{
		ContentType: aws.String("application/json"),
		Key:         aws.String(keyParts[0]),
		Body:        resp.Body,
		Bucket:      aws.String(s3bucket),
	}

	_, err = uploader.Upload(upParams)
	if err != nil {
		panic(fmt.Errorf("unable to upload %s: %s", keyParts[0], err))
	}

	localCache[href] = true
	// don't expire key for two days should we need to recover
	twoDays := time.Hour * 24 * 2
	if err := redisConn.Set(ctx, href, 1, twoDays).Err(); err != nil {
		panic(fmt.Errorf("unable to cache %s: %s", keyParts[0], err))
	}

	return false
}

func handler() error {
	resp, err := retryClient.Get(indexURL)
	if err != nil {
		panic(fmt.Errorf("unable to GET(%s): %s", indexURL, err))
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		panic(fmt.Errorf("unable to parse response: %s", err))
	}

	doc.Find("a[href]").EachWithBreak(func(i int, item *goquery.Selection) bool {
		href, _ := item.Attr("href")
		if filepath.Ext(href) == ".json" && shouldBreakForHref(href) {
			return false
		}
		return true
	})

	return nil
}

func init() {
	opt, err := redis.ParseURL(os.Getenv("REDIS_URL"))
	if err != nil {
		panic(fmt.Errorf("unable to connect to redis: %s", err))
	}
	redisConn = redis.NewClient(opt)

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	}))
	uploader = s3manager.NewUploader(sess)

	rC := retryablehttp.NewClient()
	rC.Logger = nil
	rC.RetryMax = 3
	retryClient = rC.StandardClient()
	retryClient.Timeout = 5 * time.Second
}

func main() {
	lambda.Start(handler)
}
