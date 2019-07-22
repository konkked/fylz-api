package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"os"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
)

// TODO: Make these constants configurable env variables.
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
	//jsonString, _ := json.Marshal(rq.Headers)
	//log.Println("RequestHeaders: " + string(jsonString))
	var fileContent []byte
	if rq.IsBase64Encoded {
		fileContent, _ = base64.StdEncoding.DecodeString(rq.Body)
		// if err != nil {
		// 	log.Fatal(err)
		// 	log.Println("Level=Error, Action=DecodeBase64Body, Message=Failed to decode base64 string.")
		// 	return Response{StatusCode: 500}, nil
		// }
	} else {
		fileContent = []byte(rq.Body)
	}

	id := uuid.New().String()
	filename := rq.PathParameters["filename"]

	if filename == "" {
		log.Println("Level=Error, Action=ParsingParameters, Message=Request missing required parameter filename.")
		return Response{StatusCode: 500}, nil
	}

	s, err := session.NewSession(&aws.Config{Region: aws.String(S3_REGION)})

	if err != nil {
		log.Fatal(err)
		log.Println("Level=Error, Action=CreatingS3Session, Message=Failed to create S3 Session.")
		return Response{StatusCode: 500}, nil
	}

	var contentLength int64
	contentLenStr := rq.Headers["content-length"]
	if contentLenStr == "" {
		contentLength = int64(len(fileContent))
	} else {
		contentLengthInt, err := strconv.Atoi(contentLenStr)
		if err != nil {
			log.Println("Level=Warn, Action=ParsingContentLength, Message=Failed to parse content-length header.")
			contentLength = int64(len(fileContent))
		}
		contentLength = int64(contentLengthInt)
	}

	contentType := rq.Headers["content-type"]

	log.Println("Level=Info, Action=UploadingFile, Message=Uploading File, ContentType=" + contentType + ".")
	_, err = s3.New(s).PutObject(&s3.PutObjectInput{
		Bucket:        aws.String(S3_BUCKET),
		Key:           aws.String(id + "/" + filename),
		Body:          bytes.NewReader(fileContent),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(contentLength),
	})

	if err != nil {
		log.Fatal(err)
		log.Println("Level=Error, Action=CreatingS3Session, Message=Failed to create S3 Session.")
		return Response{StatusCode: 500}, nil
	}

	body, err := json.Marshal(map[string]interface{}{
		"id": id,
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
