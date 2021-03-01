run:
	go run main.go
test:
	go test ./...
coverage:
	go test -failfast=true ./...
	go tool cover -html = cover.out
	rm cover.out
mocks:
	mockery --name=DbHandler --recursive=true --case=underscore --output=./pkg/testhelper/mocks;
