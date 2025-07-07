<p align="center">
  <br/>
  <img src="./assets/hero.jpg" alt="hero" width="600">

   <h3 align="center"><b>Audius API Server</b></h3>
   <p align="center"><i>The read API server backend for the Audius mobile apps and <a href="https://audius.co">audius.co</a></i></p>
</p>

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

```
docker compose up -d
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

You can exec into the ex-indexer container:

```
kubectl --context stage -n api get pods
kubectl --context stage -n api exec -it reindexer-fd5dd5547-z2lss -- sh
bridge es-indexer reindex
```

Or, assuming listener is running, you can connect to the postgres write DB and do:

```
NOTIFY reindex
```
