
build:
	mkdir -p ./bin
	go build -o ./bin/clipboard .

build-linux:
	mkdir -p ./bin
	GOOS=linux GOARCH=amd64 go build -o ./bin/clipboard .

docker: build-linux
	docker build -t clipboard .