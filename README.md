# Fylz Api - Simple unsercured fileshare

## ML Assignment Details

### Additional Requirement - List Files

I implemented the list files endpoint `GET api/files?[all=true]&[continuation_token]`, by default the endpoint will paginate the response and provide a continuation token, an additional query parameter can be provided to return all items (or the remaining items if a continuation token is provided).

### Bonus Endpoint

I also implemented an api that will provide the client with a signed url that can be used to fetch or upload a file directly from S3. This is useful for very large file uploads and would also cost/infastructure pressure as we'd offload long running non-cpu intensive IO work off of our lambda instances and onto AWS.

Endpoint to get a signed download url `GET api/url/download?id=<id>`
Endpoint to get a signed upload url `GET api/url/upload?filename=<filename>`

### Architectural Decisions

**Programming Language** - I decided to implement this api in golang, I figured it would be a good dry run to see if I can quickly pick up the language and build something you guys are working in, and it is a great programming language (serisouly had a lot of fun programming in it).

**App Hosting** - I wanted to maximize my flexibility and development speed, Function as a Service (FaaS) development is known for being extremely fast even though application integration can cause headaches at times thought it was worth the risk.

**CSP** - I chose AWS since it has the most mature FaaS offerings.

**Deployment** - I started off with a esoteric unknown deployment tool (aegis?) but quickly learned my lesson and hopped over to serverless.

**Dependency Management** - Initially I chose dep as my dependency manager, but after reading some comments from the community realized it made more sense to use go modules in my case.

**Storage** - Based on my choice of CSP S3 was really the only thing that made sense.

### Application Design

**UniqueId** - Chose a guid since generating them is light weight and are a widely known concept amoung developers.

**Associating the File with the UUID** - Decided to save the file to S3 with the uuid as a key prefix seperated by path delimiter `<id>/<filename>`, all operations would end up becoming extremly simple in this case. It would also be easy to extend this to a secure application by adding the username as the first prefix and performing searches only against those prefixes.

**First attempt at upload was using Multi-Part Upload** - I initially wanted to use a multipart upload to allow multiple files to be uploaded at once, AWS Api Gateway doesn't seem to play very nice with multipart form data though. I eventually gave up and implemented a single file upload endpoint as originally requested in the specs.

**Use of Signed URLs** - I initially did this as a workaround to issues I was having implementing the upload but after doing some more research I think this functionality makes a lot of sense, you save cost on compute since you aren't executing the IO over a lambda and you are no longer limited to the lambda max request size.

### Build+Deploy Instructions

See documentation below, the major dependencies that I don't think will be programmatically resolved would be npm, go, aws-cli, serverless. Docker is required to run functions locally. This is one of my first non-toy/non-trivial golang projets so possible I missed some steps in the setup.

### Deployed Endpoints

This API is currently deployed on my AWS account:

endpoints:
  PUT - https://llaqv4rff3.execute-api.us-east-1.amazonaws.com/prod/api/files/upload/{filename}
  GET - https://llaqv4rff3.execute-api.us-east-1.amazonaws.com/prod/api/files/{id}
  GET - https://llaqv4rff3.execute-api.us-east-1.amazonaws.com/prod/api/url/{action}
  GET - https://llaqv4rff3.execute-api.us-east-1.amazonaws.com/prod/api/files
functions:
  upload: fylz-api-prod-upload
  download: fylz-api-prod-download
  url: fylz-api-prod-url
  list: fylz-api-prod-list

#### Direct Upload - POST api/files/upload/{filename}

Uploads a file and provides a unique identifier that can be used to fetch it via the download endpoint.

```http
POST - https://llaqv4rff3.execute-api.us-east-1.amazonaws.com/prod/api/files/upload/{filename}
Content-Type:*
VERB PUT

```

#### Direct Download - GET api/files/{id}?redirect={?redirect}

Download a file associated with the unique identifier preserving the original filename.

```http
PATH: https://llaqv4rff3.execute-api.us-east-1.amazonaws.com/prod/api/files/{id}
VERB: GET
PARAMS: 
    id: The unique identifier presented to the user during url exchange or after direct upload.
    redirect (optional): When true the client is redirected to a signed s3 string, otherwise the file is downloaded directly
```

#### Get Signed Url - GET api/url/{upload|download}?id={?id}&filename={?filename}]

Gets a signed url that can be used to upload or download a file, response object also icludes the unique id of the file that can be used to download it after upload has completed.

```http
PATH: https://llaqv4rff3.execute-api.us-east-1.amazonaws.com/prod/api/url/{action}
VERB: GET
CONTENT-TYPE: application/json
PARAMS:
    action = upload|download
    id (required for download) = unique id of the file.
    filename (required for upload) = filename of the file to be uploaded.
```

Response Example

```javascript
{
    "expiry": "2019-07-22T04:49:06.704134647Z",
    "headers": null,
    "id": "43df2f50-1ab7-4961-9979-a96cd53d65dd",
    "url": "https://<bucket-name>.s3.amazonaws.com/43df2f50-1ab7-4961-9979-a96cd53d65dd/IMG_1174.jpeg?X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=AS...(cont.)"
}
```

#### List files - GET api/files?continuation_token={?continuation_token}&all={all}

List files in the system.

```http
PATH: https://llaqv4rff3.execute-api.us-east-1.amazonaws.com/prod/api/files
VERB: GET
CONTENT-TYPE: application/json
PARAMS: 
    continuation_token(optional) = Used to get the next page of data.
    all(optional) = When true api lists all files in the system instead of paginating.
    id (required for download) = unique id of the file.
    filename (required for upload) = filename of the file to be uploaded.
```

## Documentation

Fylz is a simple file sharing api built for a world without borders, have you ever wanted to save files somewhere but keep getting bombarded with silly queries to create an account? Well look no further, fylz is here to save the day my friend :).

## Getting Started

If you are a brave soul looking to contribute to this project the setup is fairly straight forward. You'll need to have the following dev dependencies are installed

- golang v1.12+
- [npm](https://docs.npmjs.com/getting-started/)
- [serverless](https://serverless.com/framework/docs/getting-started/)
- [aws-cli](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html)

In order to deploy an instance of the api to your account the s3Bucket name variable should be changed, besides that everything else should be there.

### Building

Fylz uses a make file to build and deploy, this is the only command you should have to run.

```shell
make build
```

### Local Deployment

 Local invocation is possible through serverless, it is triggered via the following command:

```shell
sls invoke local --function <function-name>
```

More documenation on local invocation can be found in the serverless documentation.
[AWS - Local Invoke](https://serverless.com/framework/docs/providers/aws/cli-reference/invoke-local/)

### Deploy

Deployment is handled by serverless, can be iniated via make or the serverless command.

```shell
#PCleans, builds and deploys.
make deploy

#Only deploys
sls deploy
```

#### Configuration

The serverless.yaml file contains the CloudFormation stack used to deploy all required infrastructure. In order to get the application deployed in a different environment the s3Bucket will have to be changed to a unique value  

## Using the Fylz Api

### Uploading Files

If you're ready to upload files to our system you'll need to request an upload url first (the direct upload endpoint is currently on the fritz). I know it sounds like a lot of work but it's pretty easy.

```shell
 # Grabs the upload url and code that can be used to retrieve the file later.
 http -j https://llaqv4rff3.execute-api.us-east-1.amazonaws.com/prod/api/url/upload?filename=an_awesome_file.bin >> url-upload-rsp.json
 cat url-upload-rsp.json | jq '.'

 ```

 Now use the provided upload url to push your file to the cloud!

``` shell
 rm an_awesome_file.bin
 dd bs=1024 count=50000 </dev/urandom > an_awesome_file.bin
 
 url=$(cat url-upload-rsp.json | jq '.url')
 url=$(sed -e 's/^"//' -e 's/"$//' <<<"$url")
 id=$(cat url-upload-rsp.json | jq '.id')
 id=$(sed -e 's/^"//' -e 's/"$//' <<<"$id")

 # Uploads the file to fylz api
 http -j PUT $url @an_awesome_file.bin >> upload-rsp.json
 cat upload-rsp.json | jq
```

### Downloading Files

Great, now you have to options to retreive your data, you can use the same signed url method to download 
the file directly from s3 (recommended) or you can hit our file download api.

```shell
# Using a signed url
 http -j https://llaqv4rff3.execute-api.us-east-1.amazonaws.com/prod/api/url/download?id=$id >> url-download-rsp.json
 cat url-download-rsp.json | jq '.'

 url=$(cat url-upload-rsp.json | jq '.url')
 url=$(sed -e 's/^"//' -e 's/"$//' <<<"$url")

 http $url > an_awesome_downloaded_file.bin

 # Using direct download
 http https://llaqv4rff3.execute-api.us-east-1.amazonaws.com/prod/api/files/$id > the_same_awesome_file.bin
 cmp an_awesome_file.bin the_same_awesome_file.bin
 cmp an_awesome_file.bin an_awesome_downloaded_file.bin
```

### Listing files

The list files endpoint returns a list of the items stored in files, the api supports continuation if there are more than 100 files uploaded to the service at the time.

```shell
http -j https://llaqv4rff3.execute-api.us-east-1.amazonaws.com/prod/api/files >> list-files-rsp.json
cat list-files-rsp.json | jq '.'
```

### RoadMap

I am planning on implementing the following to familarize myself with go, in order of priority.

1. Securing the Api
2. Expiring token based access control.
3. Golang client
4. UI