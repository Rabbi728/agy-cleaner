BINARY_NAME=agy-cleaner
INSTALL_DIR=$(HOME)/.local/bin

.PHONY: all build install run clean test

all: build

build:
	go build -ldflags="-s -w" -o $(BINARY_NAME) .

install: build
	mkdir -p $(INSTALL_DIR)
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed $(BINARY_NAME) to $(INSTALL_DIR)/$(BINARY_NAME)"

run:
	go run main.go

clean:
	rm -f $(BINARY_NAME)

test:
	go test -v ./...
