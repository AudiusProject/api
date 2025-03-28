
dev::
	wgo -verbose -file sqlc.yaml -file .sql -xfile .go sqlc generate :: wgo run -file .go -debounce 10ms -verbose main.go

dev2::
	modd

test::
	sqlc generate
	go test -count=1 ./...

psql::
	docker compose exec db psql -U postgres

setup::
	go install github.com/cortesi/modd/cmd/modd@latest
	go install github.com/bokwoon95/wgo@latest
	go install -v github.com/sqlc-dev/sqlc/cmd/sqlc@latest
