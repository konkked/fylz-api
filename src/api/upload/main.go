package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	//"os"
	//"path/filepath"
	//"strconv"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/google/uuid"
)

// TODO: Make these constants configurable env variables.
const (
	S3_REGION = "us-east-1"
	S3_BUCKET = "fylz-files"
)

// Response is of type APIGatewayProxyResponse since we're leveraging the
// AWS Lambda Proxy Request functionality (default behavior)
//
// https://serverless.com/framework/docs/providers/aws/events/apigateway/#lambda-proxy-integration
type Response events.APIGatewayProxyResponse
type Request events.APIGatewayProxyRequest

func htmap(s string) map[string]string {
	ret := make(map[string]string)
	kvps := strings.Split(s, ";")
	for _, kvp := range kvps {
		spl := strings.Split(kvp, "=")
		if len(spl) > 1 {
			ret[strings.Trim(spl[0], " ")] = strings.Trim(spl[1], "\" ")
		}
	}
	return ret
}

// func toUtf8(iso8859_1_buf []byte) []byte {
// 	buf := make([]rune, len(iso8859_1_buf))
// 	for i, b := range iso8859_1_buf {
// 		buf[i] = rune(b)
// 	}
// 	return []byte(string(buf))
// }

// func cleanMultipartTempFiles() {
// 	files, err := filepath.Glob(os.Getenv("TMPDIR") + "*")
// 	log.Println("TempFiles: " + strconv.Itoa(len(files)))
// 	if err != nil {
// 		panic(err)
// 	}
// 	for _, f := range files {
// 		log.Println(f)
// 		// if err := os.Remove(f); err != nil {
// 		// 	panic(err)
// 		// }
// 	}
// }

// Handler is our lambda handler invoked by the `lambda.Start` function call
func Handler(ctx context.Context, rq Request) (Response, error) {
	//jsonString, _ := json.Marshal(rq.Headers)
	//log.Println("RequestHeaders: " + string(jsonString))
	mediaType, params, err := mime.ParseMediaType(rq.Headers["content-type"])
	if !strings.HasPrefix(mediaType, "multipart/") {
		log.Println("Level=Error, Action=ParseMediaType, Message=Unsupported media type, MediaType=" + mediaType + ".")
		return Response{StatusCode: 415}, nil
	}
	//jsonString, _ = json.Marshal(params)
	//log.Println("MultipartHeaderParams:" + string(jsonString))
	log.Println("Level=Info, Action=ParseMediaType, Message=Parsing media type.")
	if err != nil {
		log.Println("Level=Error, Action=ParseMediaType, Message=Parsing media type failed.")
		log.Fatal(err)
		return Response{StatusCode: 500}, nil
	}
	log.Println("Level=Info, Action=ParseMediaType, Message=Parsed media type, Value=" + mediaType + ".")

	ids := make(map[string]string)

	log.Println("Level=Info, Action=ReadMultiPartContent, Message=Reading multipart content.")

	//cleanMultipartTempFiles()
	mr := multipart.NewReader(strings.NewReader(rq.Body), params["boundary"])
	for {
		log.Println("Level=Info, Action=ReadMultiPartContent, Message=Reading multipart content part.")
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
			return Response{StatusCode: 500}, nil
		}
		slurp, err := ioutil.ReadAll(p)
		//futile attempts to make the multipart upload behave properly
		slurp = bytes.ReplaceAll(slurp, []byte("\xEF\xBF\xBD"), []byte(""))
		id := uuid.New().String()
		filename := p.FileName()
		if err != nil {
			log.Println("Level=Error, Action=Upload, Message=Failed to encode bytes, FileName=" + filename + ", Id=" + id + ".")
			log.Fatal(err)
			return Response{StatusCode: 500}, nil
		}
		contentType := p.Header.Get("Content-Type")
		err = upload(id+"/"+filename, slurp, contentType)
		if err != nil {
			log.Println("Level=Error, Action=Upload, Message=Failed to upload file, FileName=" + filename + ", Id=" + id + ".")
			log.Fatal(err)
			return Response{StatusCode: 500}, nil
		}
		ids[filename] = id
	}
	// log.Println(rq.Body)
	// mr := multipart.NewReader(strings.NewReader(rq.Body), params["boundary"])
	// for {
	// 	p, err := mr.NextPart()
	// 	if err == io.EOF {
	// 		break
	// 	}
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	slurp, err := ioutil.ReadAll(p)
	// 	if err != nil {
	// 		log.Fatal(err)
	// 	}
	// 	log.Printf("Part %q: %q\n", p.Header.Get("Foo"), slurp)
	// }

	// if filename == "" {
	// 	log.Println("Level=Error, Action=ReadFileName,  Message=Filename was not provided.")
	// 	return Response{StatusCode: 500}, nil
	// }

	// id := uuid.New().String()
	// err := upload(id+"/"+filename, fileBytes)
	// if err != nil {
	// 	log.Println("Level=Error, Action=UploadFile, Message=Uploading file failed.")
	// 	log.Fatal(err)
	// 	return Response{StatusCode: 500}, nil
	// }

	body, err := json.Marshal(map[string]interface{}{
		"ids": ids,
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

func upload(filename string, content []byte, contentType string) error {

	// Create a single AWS session (we can re use this if we're uploading many files)
	s, err := session.NewSession(&aws.Config{Region: aws.String(S3_REGION)})
	if err != nil {
		return err
	}

	// Upload
	err = uploadToS3(s, filename, content, contentType)
	if err != nil {
		return err
	}

	return nil
}

// AddFileToS3 will upload a single file to S3, it will require a pre-built aws session
// and will set file info like content type and encryption on the uploaded file.
func uploadToS3(s *session.Session, filename string, content []byte, contentType string) error {

	// Open the file for use
	// file, err := os.Open(fileDir)
	// if err != nil {
	//     return err
	// }
	// defer file.Close()

	// Get file size and read the file content into a buffer
	// fileInfo, _ := file.Stat()
	// var size int64 = fileInfo.Size()
	// buffer := make([]byte, size)
	// file.Read(buffer)

	// Config settings: this is where you choose the bucket, filename, content-type etc.
	// of the file you're uploading.
	_, err := s3.New(s).PutObject(&s3.PutObjectInput{
		Bucket:        aws.String(S3_BUCKET),
		Key:           aws.String(filename),
		Body:          bytes.NewReader(content),
		ContentType:   aws.String(contentType),
		ContentLength: aws.Int64(int64(len(content))),
	})
	return err
}

func main() {
	lambda.Start(Handler)
}
