SOURCES := $(shell /usr/bin/find cmd internal -name '*.go')
COMPOSE := docker compose -p your-go-ekto -f deployments/compose.yaml

.PHONY: build
build: bin/ekto-server

.PHONY: run
run:
	go run github.com/lightstar/your-go-ekto/cmd/ekto-server

.PHONY: image
image:
	$(COMPOSE) build your-go-ekto

.PHONY: run-in-docker
run-in-docker:
	$(COMPOSE) up -d

.PHONY: stop-in-docker
stop-in-docker:
	$(COMPOSE) down

bin/ekto-server: $(SOURCES)
	go build -o bin github.com/lightstar/your-go-ekto/cmd/ekto-server
