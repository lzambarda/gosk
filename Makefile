.PHONY: help
help: ## Shows help messages.
	@grep -E '^[0-9a-zA-Z_-]+:(.*?## .*)?$$' $(MAKEFILE_LIST) | sed 's/^Makefile://' | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

run="."
dir="./..."
short="-short"
run="."
dir="./..."
short="-short"
timeout=20s

postgres_image=postgres
postgres_container=gosk_pg_1
postgres_data=gosk_pg_data_1


.PHONY: dependencies
dependencies: ## Install all dependencies for build and unit_test
	@go install mvdan.cc/gofumpt@latest
	@go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@go install github.com/vektra/mockery/v2@latest
	@go mod tidy

.PHONY: lint
lint: ## Runs formatters and linters, install them with make dependencies
	gofumpt -w -l .
	golangci-lint run ./...

.PHONY: mocks
mocks: ## Runs go generate to update all mocks using mockery, install it with make dependencies
	@go generate ./...

.PHONY: unit_test
unit_test: ## short="" to only disable short tests. Use run=<TestName> to run a specific test.
	@go test --timeout=40s $(short) $(dir) -run $(run);

.PHONY: integration_test_dependencies
integration_test_dependencies: ## Create the docker containers necessary to run the ingestion test
	@-docker pull $(postgres_image)
	@-docker network create gosk

	docker volume create $(postgres_data)
	docker run -d -p 5432:5432 -m 512M -e POSTGRES_PASSWORD=gosk_test --name $(postgres_container) -v $(postgres_data):/var/lib/postgresql/data $(postgres_image)
	timeout 10 bash -c "until PGPASSWORD=gosk_test psql -h localhost -p 5432 -U postgres -c 'select 1' 2>&1 | grep '(1 row)' > /dev/null ; do printf '.'; sleep 0.3; done; printf '\n'"
	docker exec --user postgres $(postgres_container) psql -U postgres -c "CREATE USER gosk_test WITH SUPERUSER LOGIN PASSWORD 'gosk_test';"
	docker exec --user postgres $(postgres_container) psql -U postgres -c "CREATE DATABASE gosk_test OWNER gosk_test;"


.PHONY: start_test_container
start_test_container: ## Start test containers.
	@-docker start $(postgres_container)
	@timeout 10 bash -c "until PGPASSWORD=gosk_test psql -h localhost -p 5432 -U postgres -c 'select 1' 2>&1 | grep '(1 row)' > /dev/null ; do printf '.'; sleep 0.3; done; printf '\n'"

.PHONY: stop_test_container
stop_test_container: ## Stop test containers.
	@-docker stop $(postgres_container)

.PHONY: reset_docker
reset_docker: ## Reset containers and delete their data.
	@-docker rm -f $(postgres_container)
	@-docker volume rm $(postgres_data)
	@-docker network rm gosk

.PHONY: integration_test
integration_test: start_test_container ## short="" to disable only short tests. Use run=<TestName> to run a specific test.
	@go mod tidy; go test -trimpath -failfast --timeout=$(timeout) -tags=integration $(short) $(dir) -run $(run) $(flags)

.PHONY: ci_test
ci_test: ## Run tests for CI.
	go test --timeout=10m -failfast -v -tags=integration -coverprofile coverage.out -covermode count  ./... 

.PHONY: check_coverage
check_coverage: ci_test
	@echo "Current test coverage : $(shell go tool cover -func=coverage.out | grep total | grep -Eo '[0-9]+\.[0-9]+') %"
