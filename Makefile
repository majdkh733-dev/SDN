.PHONY: test test-go test-web build api seed web

test: test-go test-web

test-go:
	go test $$(go list ./... | grep -v '/apps/web/')

test-web:
	npm --prefix apps/web ci
	npm --prefix apps/web run lint
	npm --prefix apps/web run typecheck
	npm --prefix apps/web test
	npm --prefix apps/web run build

build:
	go build -o bin/sdn-api ./services/api/cmd/api
	go build -o bin/sdn-worker ./services/worker/cmd/worker
	go build -o bin/sdn-agent ./connectors/site-agent

api:
	go run ./services/api/cmd/api

seed:
	go run ./services/api/cmd/api seed

web:
	npm --prefix apps/web run dev
