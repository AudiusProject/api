
dev::
	wgo -file sqlc.yaml -file .sql -xfile .go sqlc generate :: wgo run -file .go -debounce 10ms main.go

test::
	sqlc generate
	go test -count=1 -cover ./...

staging::
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/bridge-amd64
	rsync -ravz build/ stage-discovery-4:bridgerton
	ssh stage-discovery-4 -t 'cd bridgerton && docker compose up -d --build && docker compose restart bridge'

psql::
	docker compose exec db psql -U postgres

setup::
	go install github.com/bokwoon95/wgo@latest
	go install -v github.com/sqlc-dev/sqlc/cmd/sqlc@latest

apidiff::
	npx deno run -A apidiff.ts
