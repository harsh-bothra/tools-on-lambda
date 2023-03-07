APP=main
# https://medium.com/the-go-journey/adding-version-information-to-go-binaries-e1b79878f6f2
GIT_COMMIT=$(shell git rev-parse --short=10 HEAD)

.PHONY: build-and-execute
build-and-execute:
	go build -ldflags "-X main.GitCommit=${GIT_COMMIT}" -o ${APP} *.go && chmod +x ./${APP} && ./${APP}

.PHONY: deploy
deploy:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-X main.GitCommit=${GIT_COMMIT}" -o main *.go
	chmod +x create_zip.sh && ./create_zip.sh

.PHONY: run
run:
	./${APP}

.PHONY: debug
debug: 
	export DEBUG=True && make build-and-execute
	
.PHONY: prod
prod: 
	export PROD=True && make build-and-execute
