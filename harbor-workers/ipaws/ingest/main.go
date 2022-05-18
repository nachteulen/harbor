package main

import (
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-lambda-go/lambdacontext"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aws/aws-sdk-go/service/sns"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/go-retryablehttp"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	log "github.com/sirupsen/logrus"
)

var (
	pgDB        *sqlx.DB
	retryClient *http.Client
	redisConn   *redis.Client
	snsClient   *sns.SNS
	stdFields   map[string]interface{}
	uploader    *s3manager.Uploader

	ctx     = context.Background()
	traceID = ""

	awsRegion = os.Getenv("AWS_REGION")
	redisUrl  = os.Getenv("REDIS_URL")
	s3bucket  = os.Getenv("BUCKET_NAME")
	snsArn    = os.Getenv("IPAWS_SNS_ARN")

	ipawsURL = "https://apps.fema.gov/IPAWSOPEN_EAS_SERVICE/rest/public/recent/2012-08-21T11:40:43Z?pin=N3Z3Y3p3MWphajE"
)

func handler(awsCtx context.Context) error {
	setCtxFields(awsCtx)

	resp, err := retryClient.Get(ipawsURL)
	if err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"url": ipawsURL, "error": err}).
			Fatal("fetch failed")
	}
	defer resp.Body.Close()

	// get the bytes and refill the buffer for the s3 upload
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	bodyReader := ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
	if err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"url": ipawsURL, "error": err}).
			Fatal("io read failed")
	}

	// TODO Compress xml in memory before sending
	// TODO If ipaws is data we are going to use, convert to and save csvs to s3 for DW bulk loads
	if err = uploadToS3(bodyReader, "ipaws/raw/", ".xml"); err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"error": err}).
			Fatal("s3 upload failed")
	}

	alerts, err := getAlerts(bodyBytes)
	if err != nil {
		log.WithFields(stdFields).WithFields(log.Fields{"error": err}).
			Fatal("getAlerts failed")
	}

	log.WithFields(stdFields).Infof("parsing %v alerts", len(alerts.Alert))
	uniqueAlerts := 0
	for _, alert := range alerts.Alert {
		inCache, err := checkCacheForAlert(alert.Identifier)
		if err != nil {
			log.WithFields(stdFields).Fatalf("failed alert existence check: %s", err)
		}

		if inCache == false {
			if err = cacheAlert(alert); err != nil {
				redisConn.Del(ctx, alert.Identifier)
				log.WithFields(stdFields).WithFields(log.Fields{"alertId": alert.Identifier, "error": err}).
					Fatal("failed to cache alert")
			}
			log.WithFields(stdFields).WithFields(log.Fields{"alertId": alert.Identifier}).
				Info("cached alert")

			// an event can spawn multiple alerts
			shortFormAlerts, err := getShortFormAlerts(alert)
			if err != nil {
				log.WithFields(stdFields).WithFields(log.Fields{"alertId": alert.Identifier, "error": err}).Warn()
				continue
			}

			for _, sfAlert := range shortFormAlerts {
				if err := writeToAlertQueue(*sfAlert); err != nil {
					redisConn.Del(ctx, alert.Identifier)
					log.WithFields(stdFields).WithFields(log.Fields{"alertId": alert.Identifier, "error": err}).
						Fatal("failed to write to events queue")
				}
				log.WithFields(stdFields).WithFields(log.Fields{"alertId": sfAlert.Identifier}).
					Info("added alert to all alerts queue")

				if err = sendShortFormAlert(*sfAlert); err != nil {
					redisConn.Del(ctx, alert.Identifier)
					log.WithFields(stdFields).WithFields(log.Fields{"alertId": sfAlert.Identifier, "error": err}).
						Fatal("failed to publish alert")
				}
				log.WithFields(stdFields).WithFields(log.Fields{"alertId": sfAlert.Identifier}).
					Info("published alert")
			}

			uniqueAlerts = uniqueAlerts + 1
		}
	}
	log.WithFields(stdFields).Infof("processed %v unique alerts", uniqueAlerts)

	return nil
}

func uploadToS3(body io.ReadCloser, prefix string, fileExt string) error {
	s3Key := prefix + getS3KeyFromDate() + fileExt
	uploadParams := &s3manager.UploadInput{
		ContentType: aws.String("application/xml"),
		Key:         aws.String(s3Key),
		Body:        body,
		Bucket:      aws.String(s3bucket),
	}

	_, err := uploader.Upload(uploadParams)
	if err != nil {
		return err
	}
	log.WithFields(stdFields).WithFields(log.Fields{"bucket": s3bucket, "key": s3Key}).
		Info("xml event document uploaded to s3")

	return nil
}

func getS3KeyFromDate() string {
	// get standardized file name
	// remove time component and format into path
	t := time.Now()
	tf := t.Format(time.RFC3339)

	datePath := strings.ReplaceAll(strings.Split(tf, "T")[0], "-", "/")
	fullKey := datePath + "/" + tf
	return fullKey
}

func setCtxFields(awsCtx context.Context) {
	lCtx, ok := lambdacontext.FromContext(awsCtx)

	if ok {
		traceID = lCtx.AwsRequestID
	}
	stdFields = log.Fields{"traceID": traceID}
}

func init() {
	log.SetLevel(log.InfoLevel)
	log.SetFormatter(&log.JSONFormatter{
		DisableTimestamp: true,
	})
	log.SetOutput(os.Stdout)

	d, err := sqlx.Connect("postgres", os.Getenv("DB_CONN"))
	if err != nil {
		panic(err)
	}
	pgDB = d

	opt, _ := redis.ParseURL(redisUrl)
	redisConn = redis.NewClient(opt)

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(awsRegion),
	}))
	uploader = s3manager.NewUploader(sess)
	snsClient = sns.New(sess)

	rC := retryablehttp.NewClient()
	rC.Logger = nil
	rC.RetryMax = 3
	retryClient = rC.StandardClient()
	retryClient.Timeout = 5 * time.Second
}

func main() {
	lambda.Start(handler)
}
