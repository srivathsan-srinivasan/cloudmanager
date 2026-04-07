BINARY_NAME=cloudmanager
VERSION_FILE=VERSION

.PHONY: build clean run version

build: version
	@VERSION=$$(cat $(VERSION_FILE)); \
	BUILD_TIME=$$(date -u +'%Y-%m-%dT%H:%M:%SZ'); \
	go build -ldflags "-X main.Version=$$VERSION -X main.BuildTime=$$BUILD_TIME" -o $(BINARY_NAME) main.go

version:
	@if [ ! -f $(VERSION_FILE) ]; then echo "1.0.124" > $(VERSION_FILE); fi
	@awk -F. '{print $$1"."$$2"."$$3+1}' $(VERSION_FILE) > $(VERSION_FILE).tmp && mv $(VERSION_FILE).tmp $(VERSION_FILE)
	@echo "New version: $$(cat $(VERSION_FILE))"

run: build
	./$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)
