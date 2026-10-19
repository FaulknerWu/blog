SQLC_IMAGE ?= sqlc/sqlc:1.31.1
TEST_DB    ?= postgres://postgres:test@127.0.0.1:55432/postgres?sslmode=disable

.PHONY: generate build test test-db test-db-stop docker

generate:
	docker run --rm -u $$(id -u):$$(id -g) -v "$$PWD":/src -w /src $(SQLC_IMAGE) generate

build:
	CGO_ENABLED=0 go build -trimpath -tags nodynamic -o bin/blogd ./cmd/blogd
	CGO_ENABLED=0 go build -trimpath -o bin/blog ./cmd/blog

test:
	BLOG_TEST_DATABASE_URL='$(TEST_DB)' go test ./...

test-db:
	docker run -d --name blog-api-test-pg -e POSTGRES_PASSWORD=test \
		-e POSTGRES_INITDB_ARGS="--encoding=UTF8 --locale=C.UTF-8" \
		-p 127.0.0.1:55432:5432 postgres:17

test-db-stop:
	docker rm -f blog-api-test-pg

docker:
	docker build -t blog-api .
