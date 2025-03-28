
dev::
	modd

test::
	sqlc generate
	go test -count=1 ./...

psql::
	docker compose exec db psql -U postgres

setup::
	go install github.com/cortesi/modd/cmd/modd@latest
	go install -v github.com/sqlc-dev/sqlc/cmd/sqlc@latest
