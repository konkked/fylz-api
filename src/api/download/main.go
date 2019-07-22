package main

import (
	"context"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// TODO: Replace with environment variables.
var (
	S3_REGION = os.Getenv("S3_REGION")
	S3_BUCKET = os.Getenv("S3_BUCKET")
)

// Response is of type APIGatewayProxyResponse since we're leveraging the
// AWS Lambda Proxy Request functionality (default behavior)
//
// https://serverless.com/framework/docs/providers/aws/events/apigateway/#lambda-proxy-integration

type Response events.APIGatewayProxyResponse
type Request events.APIGatewayProxyRequest

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(ctx context.Context, rq Request) (Response, error) {
	id := rq.PathParameters["id"]
	redirect := rq.QueryStringParameters["redirect"]

	log.Println("Level=Info, Action=HandlingRequest, Message=Handling download request, Parameters.Id=" + id + ", Parameters.Redirect" + redirect + ".")

	log.Println("Level=Info, Action=CreatingS3Session, Message=Creating S3 session.")
	// Create a single AWS session (we can re use this if we're uploading many files)
	s, err := session.NewSession(&aws.Config{Region: aws.String(S3_REGION)})
	svc := s3.New(s)

	if err != nil {
		log.Println("Level=Error, Action=GetSignedUrl, Message=S3 Session creation failed.")
		return Response{StatusCode: 500}, err
	}

	if redirect == "true" {

		url, err := getSignedFileUrl(svc, id)
		if err != nil {
			log.Println("Level=Error, Action=GetSignedUrl, Message=Signed Url fetch failed.")
			return Response{StatusCode: 500}, err
		}

		if url == "" {
			log.Println("Level=Warn, Action=GetSignedUrl, Message=Item not found.")
			return Response{StatusCode: 404}, nil
		}

		resp := Response{
			StatusCode: 301,
			Headers: map[string]string{
				"Location": url,
			},
		}

		return resp, nil
	}

	fileKey, err := getFileKey(svc, id)

	log.Println("Level=Info, Action=DownloadFile, Message=Found file key, FileKey=" + fileKey + ".")

	if err != nil {
		log.Println("Level=Error, Action=DownloadFile, Message=Error thrown while fetching file key.")
		log.Fatal(err)
		return Response{StatusCode: 500}, err
	}

	if fileKey == "" {
		log.Println("Level=Warn, Action=DownloadFile, Message=Signed Url fetch failed.")
		return Response{StatusCode: 404}, err
	}

	log.Println("Level=Info, Action=DownloadFile, Message=Splitting file key, FileKey=" + fileKey + ".")
	spl := strings.Split(fileKey, "/")
	filename := spl[1]

	log.Println("Level=Info, Action=DownloadFile, Message=Getting S3 Object, FileKey=" + fileKey + ".")
	obj, _ := svc.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(S3_BUCKET),
		Key:    aws.String(fileKey),
	})
	log.Println("Level=Info, Action=DownloadFile, Message=Got S3 Object, FileKey=" + fileKey + ".")
	downloader := s3manager.NewDownloader(s)
	buff := &aws.WriteAtBuffer{}
	downloader.Download(buff, &s3.GetObjectInput{
		Bucket: aws.String(S3_BUCKET),
		Key:    aws.String(fileKey),
	})

	log.Println("Level=Info, Action=DownloadFile, Message=Initiated Download, FileKey=" + fileKey + ".")
	//strippedBytes := bytes.ReplaceAll(buff.Bytes(), []byte("\xEF\xBF\xBD"), []byte(""))
	var contentType string
	contentBytes := buff.Bytes()
	if obj.ContentType == nil {
		contentType = http.DetectContentType(contentBytes)
	} else {
		contentType = *obj.ContentType
	}

	var contentLength int64
	if obj.ContentLength == nil {
		contentLength = int64(len(contentBytes))
	} else {
		contentLength = *obj.ContentLength
	}

	log.Println("Level=Info, Action=DownloadFile, Message=Checking reponse values, ContentType=" + strconv.FormatInt(*obj.ContentLength, 10))
	return Response{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Disposition": "attachment; filename=" + filename,
			"Content-Type":        contentType,
			"Content-Length":      strconv.FormatInt(contentLength, 10),
		},
		Body:            base64.StdEncoding.EncodeToString(contentBytes),
		IsBase64Encoded: true,
	}, nil
}

func getFileKey(svc *s3.S3, id string) (string, error) {
	log.Println("Level=Info, Action=ListS3Objects, Message=Listing S3 Objects.")
	prefix := id + "/"
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(S3_BUCKET),
		Prefix: aws.String(prefix),
	}

	files, err := svc.ListObjectsV2(params)
	log.Println("Level=Info, Action=ListS3Objects, Message=List S3 Object request complete.")

	if err != nil {
		log.Fatal(err)
		return "", err
	}

	contentsLen := len(files.Contents)
	if contentsLen == 0 {
		return "", nil
	}

	log.Println("Level=Info, Action=ListS3Objects, Message=Finding first non-metadata file, ContentsLen=" + strconv.Itoa(contentsLen) + ".")
	fileKey := ""
	for _, s3Item := range files.Contents {
		log.Println("Level=Info, Action=VisitS3Objects, Message=Visited Object, Key=" + *s3Item.Key + ".")
		if !strings.HasPrefix(*s3Item.Key, id+"/.meta.") {
			fileKey = *s3Item.Key
			break
		}
	}

	if fileKey == "" {
		return "", nil
	}

	return fileKey, nil
}

func getSignedFileUrl(svc *s3.S3, id string) (string, error) {
	fileKey, err := getFileKey(svc, id)
	if err != nil {
		return "", err
	}

	req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(S3_BUCKET),
		Key:    aws.String(fileKey),
	})

	urlStr, err := req.Presign(15 * time.Minute)
	if err != nil {
		return "", err
	}

	return urlStr, nil
}

func main() {
	lambda.Start(Handler)
}
