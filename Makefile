
dev::
	wgo -file sqlc.yaml -file .sql -xfile .go sqlc generate :: wgo run -file .go -debounce 10ms main.go

test::
	sqlc generate
	go test -count=1 -cover ./...

staging::
	mkdir -p build/staging
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/staging/bridge-amd64
	rsync -ravz build/staging/ stage-discovery-4:bridgerton
	ssh stage-discovery-4 -t 'cd bridgerton && docker compose up -d --build && docker compose restart bridge'
	curl 'https://bridgerton.staging.audius.co'

production::
	mkdir -p build/production
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/production/bridge-amd64
	rsync -ravz build/production/ prod-discovery-4:bridgerton
	ssh prod-discovery-4 -t 'cd bridgerton && docker compose up -d --build && docker compose restart bridge'
	curl 'https://bridgerton.audius.co'

psql::
	docker compose exec db psql -U postgres

setup::
	go install github.com/bokwoon95/wgo@latest
	go install -v github.com/sqlc-dev/sqlc/cmd/sqlc@latest

apidiff::
	open http://localhost:1323/apidiff.html
