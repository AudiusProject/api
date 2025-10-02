export GIT_SHA := $(shell git rev-parse HEAD)

dev::
	wgo -file sqlc.yaml -file .sql -xfile .go sqlc generate :: wgo run -file .go -file .yaml -debounce 10ms main.go

pretty::
	go run cmd/pretty/main.go

indexer::
	wgo run -file .go -debounce 10ms main.go indexer

solana-indexer::
	wgo run -file .go -debounce 10ms main.go solana-indexer

up: dev

migrate::
	go run main.go migrate

test::
	sqlc generate
	go test -count=1 -cover ./...

esindexer-reindex-stage::
	kubectl --context stage -n api exec -it $(kubectl --context stage -n api get pods --no-headers -o custom-columns=":metadata.name" | grep reindexer) -- bridge es-indexer drop all

esindexer-reindex-prod::
	kubectl --context prod -n api exec -it $(kubectl --context prod -n api get pods --no-headers -o custom-columns=":metadata.name" | grep reindexer) -- bridge es-indexer drop all

psql::
	docker compose exec db psql -U postgres

setup::
	go install github.com/bokwoon95/wgo@v0.5.11
	go install -v github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0

apidiff::
	open http://localhost:1323/apidiff.html

test-schema::
	@set -a; \
	. .env; \
    if [ -z "$$writeDbUrl" ]; then \
		echo "writeDbUrl is not set in .env - using test db and running migrations"; \
		writeDbUrl=postgresql://postgres:example@localhost:21300/postgres; \
		make migrate; \
	fi; \
	adjustedUrl=$$(echo "$$writeDbUrl" | sed 's/localhost/host.docker.internal/g'); \
	docker compose exec db bash -c "pg_dump '$$adjustedUrl' --schema-only --no-owner --no-acl > ./sql/01_schema.sql"; \
	sed '/^\\restrict /d;/^\\unrestrict /d' ./sql/01_schema.sql > ./sql/01_schema.sql.tmp && mv ./sql/01_schema.sql.tmp ./sql/01_schema.sql; \
	echo "schema dumped to ./sql/01_schema.sql"
