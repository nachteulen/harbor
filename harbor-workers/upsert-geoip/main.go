package main

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/hashicorp/go-retryablehttp"
	mx "github.com/oschwald/maxminddb-golang"
)

const (
	fileName = "GeoIP2-City.mmdb"
	fileDest = "/tmp/" + fileName
	url      = "https://download.maxmind.com/app/geoip_download"
)

var (
	bucketName  string
	dyDB        *dynamodb.DynamoDB
	geoIPTable  = "geoip_db_version"
	licenseKey  string
	retryClient *http.Client
	uploader    *s3manager.Uploader
)

type upsertParams struct {
	shouldUpsert bool
	sha          string
}

func getUpsertParams(req *http.Request) *upsertParams {
	resp, err := retryClient.Do(req)
	if err != nil {
		fmt.Printf("unable to check GeoIP sha: %s\n", err)
		return &upsertParams{shouldUpsert: true}
	}

	defer resp.Body.Close()
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Printf("unable to read GeoIP sha: %s\n", err)
		return &upsertParams{shouldUpsert: true}
	}

	parts := strings.Split(string(b), " ")
	if len(parts) == 0 {
		fmt.Printf("unable to parse GeoIP sha: %s\n", string(b))
		return &upsertParams{shouldUpsert: true}
	}

	result, err := dyDB.GetItem(&dynamodb.GetItemInput{
		TableName: &geoIPTable,
		Key:       map[string]*dynamodb.AttributeValue{"sha": {S: &parts[0]}},
	})
	if err != nil {
		fmt.Printf("error getting %s: %s\n", parts[0], err)
	}

	if result.Item == nil {
		return &upsertParams{shouldUpsert: true, sha: parts[0]}
	}

	return &upsertParams{shouldUpsert: false}
}

func handler() error {
	req, _ := http.NewRequest("GET", url, nil)
	q := req.URL.Query()
	q.Add("edition_id", "GeoIP2-City")
	q.Add("license_key", licenseKey)
	q.Add("suffix", "tar.gz.sha256")
	req.URL.RawQuery = q.Encode()

	upsertParams := getUpsertParams(req)
	if !upsertParams.shouldUpsert {
		return nil
	} else {
		fmt.Printf("updating to latest db(%s)\n", upsertParams.sha)
	}

	q.Set("suffix", "tar.gz")
	req.URL.RawQuery = q.Encode()

	resp, err := retryClient.Do(req)
	if err != nil {
		return fmt.Errorf("unable to get GeoIP download: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("unable to download GeoIP updates: %d", resp.StatusCode)
	}

	if err = untar(resp.Body); err != nil {
		return fmt.Errorf("unable to untar response: %s", err)
	}

	mDB, err := mx.Open(fileDest)
	if err != nil {
		return fmt.Errorf("unable to open mx db: %s", err)
	}

	if err := mDB.Verify(); err != nil {
		return fmt.Errorf("unable to verify db: %s", err)
	}
	mDB.Close()

	f, err := os.Open(fileDest)
	if err != nil {
		return fmt.Errorf("unable to reopen %s: %s", fileDest, err)
	}
	defer f.Close()

	dest, err := os.Create("/mnt/efs/" + fileName)
	if err != nil {
		fmt.Printf("error creating dest: %s\n", err)
	} else {
		_, err = io.Copy(dest, f)
		if err != nil {
			fmt.Printf("error copying: %s\n", err)
		}
	}
	defer dest.Close()

	f.Seek(0, io.SeekStart)
	_, err = uploader.Upload(&s3manager.UploadInput{
		Body:   f,
		Bucket: aws.String(bucketName),
		Key:    aws.String(fileName),
	})
	if err != nil {
		return fmt.Errorf("unable to upload(%s): %s", fileDest, err)
	}

	ttl := time.Now().UTC().Add(14 * 24 * time.Hour).Unix()
	ttlStr := strconv.FormatInt(ttl, 10)
	_, err = dyDB.PutItem(&dynamodb.PutItemInput{
		TableName: &geoIPTable,
		Item: map[string]*dynamodb.AttributeValue{
			"sha": {S: &upsertParams.sha},
			"ttl": {N: &ttlStr},
		},
	})
	if err != nil {
		fmt.Printf("unable to put latest db sha(%s): %s\n", upsertParams.sha, err)
	}

	return nil
}

func untar(r io.Reader) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	for {
		header, err := tr.Next()

		switch {
		case err == io.EOF:
			return nil
		case err != nil:
			return err
		case header == nil:
			continue
		case filepath.Ext(header.Name) != ".mmdb":
			continue
		}

		f, err := os.OpenFile(
			fileDest,
			os.O_CREATE|os.O_RDWR,
			os.FileMode(header.Mode),
		)
		if err != nil {
			return err
		}

		if _, err := io.Copy(f, tr); err != nil {
			return err
		}

		f.Close()
	}
}

func init() {
	required := map[string]*string{
		"BUCKET_NAME": &bucketName,
		"LICENSE_KEY": &licenseKey,
	}
	for k, v := range required {
		tmp := os.Getenv(k)
		if len(tmp) == 0 {
			panic(fmt.Sprintf("%s is required", k))
		}
		*v = tmp
	}

	rC := retryablehttp.NewClient()
	rC.Logger = nil
	rC.RetryMax = 3
	retryClient = rC.StandardClient()
	retryClient.Timeout = 20 * time.Second

	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("AWS_REGION")),
	}))
	dyDB = dynamodb.New(sess)
	uploader = s3manager.NewUploader(sess)
}

func main() {
	lambda.Start(handler)
}
