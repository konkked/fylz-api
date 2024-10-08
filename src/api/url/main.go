package main

/**
* Creates a signed download and upload url, the upload url payload contains
* a unique identifier that can be used to retrieve the file at a later time.
 */

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
)

var (
	S3_REGION = os.Getenv("S3_REGION")
	S3_BUCKET = os.Getenv("S3_BUCKET")
)

const (
	MAX_URL_TTL_MINUTES     = 30
	MIN_URL_TTL_MINUTES     = 1
	DEFAULT_URL_TTL_MINUTES = 5
)

type Response events.APIGatewayProxyResponse
type Request events.APIGatewayProxyRequest

func createDownloadUrl(svc *s3.S3, id string, ttl int) (string, error) {
	fileKey, err := findFileKey(svc, id)
	if err != nil {
		return "", err
	}

	req, _ := svc.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(S3_BUCKET),
		Key:    aws.String(fileKey),
	})

	urlStr, err := req.Presign(time.Minute * time.Duration(ttl))
	if err != nil {
		return "", err
	}

	return urlStr, nil
}

func findFileKey(svc *s3.S3, id string) (string, error) {
	log.Println("Level=Info, Action=ListS3Objects, Message=Listing S3 Objects.")
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(S3_BUCKET),
		Prefix: aws.String(id + "/"),
	}

	files, err := svc.ListObjectsV2(params)

	if err != nil {
		log.Fatal(err)
		return "", err
	}

	if len(files.Contents) == 0 {
		return "", nil
	}

	fileKey := ""
	for _, s3Item := range files.Contents {
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

func parseUrlTtl(ttlPtr *string) int {
	ttl := *ttlPtr
	if ttl == "" {
		log.Printf("Level=Info, Action=ParsingUrlTtl, Message=Ttl not provided, using default %d minutes.", DEFAULT_URL_TTL_MINUTES)
		return DEFAULT_URL_TTL_MINUTES
	}
	ttlInt, err := strconv.Atoi(ttl)
	if err != nil {
		log.Printf("Level=Warn, Action=ParsingUrlTtl, Message=Invalid url ttl provided, using default %d minutes.", DEFAULT_URL_TTL_MINUTES)
		return DEFAULT_URL_TTL_MINUTES
	}
	if ttlInt > MAX_URL_TTL_MINUTES {
		log.Printf("Level=Warn, Action=ParsingUrlTtl, Message=Requested ttl is larger than allowed max, using max %d minutes.", MAX_URL_TTL_MINUTES)
		return MAX_URL_TTL_MINUTES
	}
	if ttlInt < MIN_URL_TTL_MINUTES {
		log.Printf("Level=Warn, Action=ParsingUrlTtl, Message=Requested ttl is smaller than allowed min, using min %d minutes.", MIN_URL_TTL_MINUTES)
		return MIN_URL_TTL_MINUTES
	}
	return DEFAULT_URL_TTL_MINUTES
}

func handleDownloadUrlRq(ctx context.Context, rq Request, svc *s3.S3, ttl int) (Response, error) {
	id := rq.QueryStringParameters["id"]

	if id == "" {
		log.Println("Level=Error, Action=GetSignedDownloadUrl, Message=Missing required id parameter.")
		return Response{StatusCode: 400}, nil
	}

	url, err := createDownloadUrl(svc, id, ttl)
	if err != nil {
		log.Println("Level=Error, Action=GetSignedDownloadUrl, Message=Signed Url fetch failed.")
		return Response{StatusCode: 500}, err
	}

	if url == "" {
		log.Println("Level=Warn, Action=GetSignedDownloadUrl, Message=Item not found.")
		return Response{StatusCode: 404}, nil
	}

	expiry := time.Now().Add(time.Minute * time.Duration(ttl))
	body, _ := json.Marshal(map[string]interface{}{
		"id":     id,
		"url":    url,
		"expiry": expiry,
	})

	var buf bytes.Buffer
	json.HTMLEscape(&buf, body)

	resp := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            buf.String(),
		Headers: map[string]string{
			"Content-Type":    "application/json",
			"x-flyz-fn-reply": "upload-handler",
		},
	}

	return resp, nil
}

func handleUploadUrlRq(ctx context.Context, rq Request, svc *s3.S3, ttl int) (Response, error) {
	log.Println("Level=Info, Action=CreatingS3Session, Message=Creating S3 session.")
	filename := rq.QueryStringParameters["filename"]

	if filename == "" {
		log.Println("Level=Error, Action=GenerateUploadUrl, Message=Request is missing required parameter, Parameter=filename.")
		return Response{StatusCode: 500}, nil
	}

	id := uuid.New().String()
	key := id + "/" + filename
	expiry := time.Now().Add(time.Minute * time.Duration(ttl))
	log.Println("Level=Info, Action=GenerateUploadUrl, Message=Creating signed url.")
	log.Printf("Level=Info, Action=GenerateUploadUrl, Parameters.Filename=%s, Parameters.Id=%s.", filename, id)
	uploadRq, _ := svc.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(S3_BUCKET),
		Key:    aws.String(key),
	})
	url, _, err := uploadRq.PresignRequest(expiry.Sub(time.Now()))

	if err != nil {
		log.Println("Level=Error, Action=GenerateUploadUrl, Message=Signing url failed.")
		log.Fatal(err)
		return Response{StatusCode: 500}, nil
	}

	body, _ := json.Marshal(map[string]interface{}{
		"id":     id,
		"url":    url,
		"expiry": expiry,
	})

	if err != nil {
		log.Println("Level=Error, Action=MarshalResponseJSON, Message=Marshaling json response failed.")
		log.Fatal(err)
		return Response{StatusCode: 500}, nil
	}

	var buf bytes.Buffer
	json.HTMLEscape(&buf, body)

	resp := Response{
		StatusCode:      200,
		IsBase64Encoded: false,
		Body:            buf.String(),
		Headers: map[string]string{
			"Content-Type":    "application/json",
			"x-flyz-fn-reply": "upload-handler",
		},
	}
	return resp, nil
}

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(ctx context.Context, rq Request) (Response, error) {
	action := rq.PathParameters["action"]
	ttl := rq.QueryStringParameters["ttl"]
	ttlInt := parseUrlTtl(&ttl)

	log.Println("Level=Info, Action=HandleRequest, Message=Request Start, Parameter.Action=" + action + ".")
	log.Println("Level=Info, Action=CreatingS3Session, Message=Creating session.")
	// Create a single AWS session (we can re use this if we're uploading many files)
	s, err := session.NewSession(&aws.Config{Region: aws.String(S3_REGION)})

	if err != nil {
		log.Println("Level=Error, Action=CreatingS3Session, Message=Failed to create session.")
		log.Fatal(err)
		return Response{StatusCode: 500}, nil
	}

	svc := s3.New(s)

	if strings.EqualFold("download", action) {
		return handleDownloadUrlRq(ctx, rq, svc, ttlInt)
	}

	if strings.EqualFold("upload", action) {
		return handleUploadUrlRq(ctx, rq, svc, ttlInt)
	}

	return Response{StatusCode: 400}, nil
}

func main() {
	lambda.Start(Handler)
}
