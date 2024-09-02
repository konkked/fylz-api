.PHONY: build clean deploy
GOBUILDFLAGS = -ldflags="-s -w"
GOBUILD = env GOOS=linux go build

build:
	$(GOBUILD) $(GOBUILDFLAGS) -o bin/api/upload src/api/upload/main.go
	$(GOBUILD) $(GOBUILDFLAGS) -o bin/api/download src/api/download/main.go
	$(GOBUILD) $(GOBUILDFLAGS) -o bin/api/url src/api/url/main.go
	$(GOBUILD) $(GOBUILDFLAGS) -o bin/api/list src/api/list/main.go
clean:
	rm -rf ./bin

deploy: clean build
	sls deploy --verbose
