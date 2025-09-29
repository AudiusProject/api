# Audius API Server
The API backend for the Audius mobile apps and [audius.co](https://audius.co)

[![license](https://img.shields.io/github/license/AudiusProject/api)](https://github.com/AudiusProject/api/blob/main/LICENSE) [![releases](https://img.shields.io/github/v/release/AudiusProject/api)](https://github.com/AudiusProject/api/releases/latest)

## Running

### Server

1. Create `.env` file:

   ```
   readDbUrl='postgresql://postgres:somepassword@someip:5432/audius_discovery'
   ```

   Regular database dumps are posted to S3, and can be pulled with

   ```
   curl https://audius-pgdump.s3-us-west-2.amazonaws.com/discProvProduction.dump -O
   pg_restore -d <your-database-url> \
      --username postgres \
      --no-privileges \
      --clean --if-exists --verbose -j 8 \
      discProvProduction.dump
   ```

   (more env vars and their defaults can be found in `config.go`)

2. Run `make setup` (the first time)

   ```
   make setup
   ```

3. Run `make`

   ```
   make
   ```

   http://localhost:1323/v1/users/handle/audius

   This will watch sql files + re-run `sqlc generate` + restart server when go files change.

### Tests

#### To run tests against the existing schemas
```
docker compose up -d
make test
```

#### To run tests with custom migrations

Launch a postgres instance based on the standard schema
```
docker run --rm --name postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=audius_discovery \
  -v ./sql/01_schema.sql:/docker-entrypoint-initdb.d/01_schema.sql:ro \
  -p 5432:5432 \
  -d postgres
```

Add writeDbUrl and runMigrations to your .env
```
writeDbUrl=postgresql://postgres:postgres@localhost:5432/audius_discovery
runMigrations=true
```

Update the schema and run tests
```
make migrate
docker compose up -d
make test-schema
make test
```

### Build

```
go build -o api main.go
```

## API diff

Tool for comparing the new API server endpoints with the legacy Discovery Node APIs

http://localhost:1323/apidiff.html

## Adminer

Tool for interacting with the postgres server

http://localhost:21301/?pgsql=db&username=postgres

## Schema dump

```
docker compose exec db bash
export discoveryDbUrl='a_db_url'
pg_dump $discoveryDbUrl --schema-only --no-comments --no-owner --no-acl > ./sql/01_schema.sql
```

If you re-dump schema, reset dev postgres state:

```
docker compose down --volumes
docker compose up -d
```

## ElasticSearch

ElasticSearch is configured by the env var `elasticsearchUrl`.

Tool for interacting with search

http://localhost:1323/searchtest.html

Re-index collections from scratch:

```
go build -o api main.go
time ./api reindex
```

You can also specify specific indexes. If you change the mapping you can add `drop`:

```
go build -o api main.go
time ./api reindex drop playlists
```

### Re-index in stage or prod

If you make mapping changes, or want to re-index everything, you can do:

```
make esindexer-reindex-stage

#or

esindexer-reindex-prod
```

A deploy might disrupt this command, in which case, run it again.

See `Makefile` for more details.
