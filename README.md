# Audius API Server

The read API server backend for the Audius mobile apps and [audius.co](https://audius.co) website.

## Running

### Server

1. Create `.env` file:

   ```
   discoveryDbUrl='postgresql://postgres:somepassword@someip:5432/audius_discovery'
   ```

   Other env vars:

   ```
   delegatePrivateKey: key to sign stream/download requests with
   axiomToken: axiom api token to pipe logs to axiom
   axiomDataset: axiom dataset name
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

   http://localhost:1323/v2/users/stereosteve

   This will watch sql files + re-run `sqlc generate` + restart server when go files change.

### Tests

```
docker compose up -d
make test
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
pg_dump $discoveryDbUrl --schema-only --no-owner --no-acl > ./sql/schema1.sql
```

If you re-dump schema, reset dev postgres state:

```
docker compose down --volumes
docker compose up -d
```
