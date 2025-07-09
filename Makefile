
dev::
	wgo -file sqlc.yaml -file .sql -xfile .go sqlc generate :: wgo run -file .go -file .yaml -debounce 10ms main.go

solana-indexer::
	wgo run -file .go -debounce 10ms main.go solana-indexer

up: dev

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
