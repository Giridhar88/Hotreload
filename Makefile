.PHONY: build test demo clean

# Build the hotreload binary
build:
	go build -o ./bin/hotreload .

# Run all tests
test:
	go test -v ./...

# Demo: run hotreload watching the test server
demo: build
	./bin/hotreload \
		--root ./testserver \
		--build "go build -o ./bin/testserver ./testserver" \
		--exec "./bin/testserver"

# Clean build artifacts
clean:
	rm -rf ./bin
