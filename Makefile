.PHONY: build run test lint clean

BINARY=cypher-shell-browser

build:
	go build -o $(BINARY) ./cmd/cypher-shell-browser

run: build
	./$(BINARY)

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
