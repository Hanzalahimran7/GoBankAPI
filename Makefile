build:
	@go build -o bin/gobank

run:
	@go build -o bin/gobank && ./bin/gobank

test:
	@go test -v ./...