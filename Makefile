build:
	go build -o bin/fare -v ./cmd/cli/main.go
test:
	go test -cover ./...
benchmem:
	go test -race -cpu=1,2,4 -bench . -benchmem ./...
run:
	./bin/fare -input data/path.csv -output fares.csv -c 5
