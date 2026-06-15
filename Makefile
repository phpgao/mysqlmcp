.PHONY: build test clean install run fmt docker docker-run

BINARY := bin/mysqlmcp
IMAGE  := phpgao/mysqlmcp:latest

build:
	go build -o $(BINARY) .

test:
	go test ./...

clean:
	rm -rf bin/

install:
	go install .

run:
	go run .

run-http:
	go run . -transport http

fmt:
	go fmt ./...

docker:
	docker build -t $(IMAGE) .

docker-run:
	docker run -it --rm \
	  -e MYSQLMCP_TOKEN=your-token \
	  -v $$(pwd)/config.yaml:/config.yaml:ro \
	  $(IMAGE) -config /config.yaml
