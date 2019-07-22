.PHONY: build clean deploy
GOBUILDFLAGS = -ldflags="-s -w"
GOBUILD = env GOOS=linux go build

build:
	$(GOBUILD) $(GOBUILDFLAGS) -o bin/upload src/upload/main.go
	$(GOBUILD) $(GOBUILDFLAGS) -o bin/download src/download/main.go
clean:
	rm -rf ./bin

deploy: clean build
	sls deploy --verbose
