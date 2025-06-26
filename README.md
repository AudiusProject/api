<p align="center">
  <br/>
  <img src="./hero.jpg" alt="hero" width="600">

  <br/>

  <p align="center">
    <b>Audius API Server</b>
    <br/>
    <br/>
    <i>The read API server backend for the Audius mobile apps and <a href="https://audius.co">audius.co</a></i>
  </p>
</p>

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
pg_dump $discoveryDbUrl --schema-only --no-comments --no-owner --no-acl > ./sql/01_schema.sql
```

If you re-dump schema, reset dev postgres state:

```
docker compose down --volumes
docker compose up -d
```

## ElasticSearch

Currently esindexer is running in listen mode on the same box as elasticsearch, running in docker compose in the `bridgerton` folder.

To deploy:

```
make esindexer-staging
make esindexer-production
```

To re-index collections from scratch you can:

```
ssh stage-elasticsearch
cd bridgerton
time ./bridge-amd64 reindex
```

You can also specify specific indexes. If you change the mapping you can add `drop`:

```
ssh stage-elasticsearch
cd bridgerton
time ./bridge-amd64 reindex drop playlists
```
