export GIT_SHA := $(shell git rev-parse HEAD)

dev::
	wgo -file sqlc.yaml -file .sql -xfile .go sqlc generate :: wgo run -file .go -file .yaml -debounce 10ms main.go

pretty::
	go run cmd/pretty/main.go

build-reward-codes::
	go build -o bin/create-reward-codes cmd/create_reward_codes/main.go

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
	@set -a -e; \
	writeDbUrl='postgresql://postgres:example@localhost:21300/postgres?sslmode=disable'; \
	echo "\033[0;32mBringing down any existing containers to start fresh...\033[0m"; \
	docker compose down --volumes; \
	docker compose up -d --wait; \
	echo "\n\033[0;32mRunning migrations on fresh instance...\033[0m"; \
	make migrate; \
	echo "\033[0;32mDumping schema...\033[0m"; \
	adjustedUrl=$$(echo "$$writeDbUrl" | sed 's/localhost/host.docker.internal/g'); \
	docker compose exec db bash -c "pg_dump '$$adjustedUrl' --schema-only --no-owner --no-acl > ./sql/01_schema.sql"; \
	sed '/^\\restrict /d;/^\\unrestrict /d' ./sql/01_schema.sql > ./sql/01_schema.sql.tmp && mv ./sql/01_schema.sql.tmp ./sql/01_schema.sql; \
	echo "\033[0;32mDumping migration tracker rows...\033[0m"; \
	docker compose exec db bash -c "pg_dump '$$adjustedUrl' --data-only --no-owner --no-acl --table=schema_version > ./sql/03_migration_tracker.sql"; \
	sed '/^\\restrict /d;/^\\unrestrict /d' ./sql/03_migration_tracker.sql > ./sql/03_migration_tracker.sql.tmp && mv ./sql/03_migration_tracker.sql.tmp ./sql/03_migration_tracker.sql; \
	echo "Schema dumped to ./sql/01_schema.sql and ./sql/03_migration_tracker.sql"; \
	echo "\n\033[0;32mRestarting containers...\033[0m"; \
	docker compose down --volumes; \
	docker compose up -d --wait; \
	echo "\n\033[0;32mDone\033[0m";