.PHONY: build test vet lint clean

build:
	go build ./...

test:
	go test ./...

vet:
	go vet ./...

lint: vet

clean:
	rm -f bin/wb
	go clean ./...
