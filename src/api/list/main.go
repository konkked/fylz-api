package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

// TODO: Make these constants configurable env variables.
const (
	S3_REGION = "us-east-1"
	S3_BUCKET = "fylz-files"
)

type Response events.APIGatewayProxyResponse
type Request events.APIGatewayProxyRequest

type FileResult struct {
	Id       string `json:"id"`
	FileName string `json:"filename"`
	Size     int64  `json:"size"`
}

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(ctx context.Context, rq Request) (Response, error) {
	log.Println("Level=Info, Action=CreatingS3Session, Message=Creating session.")
	// Create a single AWS session (we can re use this if we're uploading many files)
	s, err := session.NewSession(&aws.Config{Region: aws.String(S3_REGION)})

	if err != nil {
		log.Println("Level=Error, Action=CreatingS3Session, Message=Failed to create session.")
		log.Fatal(err)
		return Response{StatusCode: 500}, nil
	}

	svc := s3.New(s)

	log.Println("Level=Info, Action=ListS3Objects, Message=Listing S3 Objects.")
	params := &s3.ListObjectsV2Input{
		Bucket: aws.String(S3_BUCKET),
	}

	files, err := svc.ListObjectsV2(params)

	if err != nil {
		log.Println("Level=Info, Action=ListS3Objects, Message=Failed to list S3 Objects.")
		log.Fatal(err)
		return Response{StatusCode: 500}, err
	}

	if len(files.Contents) == 0 {
		return Response{StatusCode: 404}, nil
	}

	items := make([]FileResult, len(files.Contents))
	index := 0
	for _, s3Item := range files.Contents {
		keyparts := strings.Split("/", *s3Item.Key)
		if len(keyparts) < 2 {
			continue
		}
		var item FileResult
		item.Id = keyparts[0]
		item.FileName = keyparts[1]
		item.Size = *s3Item.Size
		items[index] = item
	}

	body, err := json.Marshal(map[string]interface{}{
		"items":                   items,
		"next_continuation_token": files.NextContinuationToken,
		"is_truncated":            files.IsTruncated,
	})

	if err != nil {
		log.Println("Level=Error, Action=MarshalResponseJSON, Message=Marshaling json response failed.")
		log.Fatal(err)
		return Response{StatusCode: 404}, nil
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

func main() {
	lambda.Start(Handler)
}
