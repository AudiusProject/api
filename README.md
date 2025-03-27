# bridgerton

run tests:

```
docker compose up -d
make test
```

run server:

create `.env` file:

```
discoveryDbUrl='postgresql://postgres:somepassword@someip:5432/audius_discovery'
```

```
make setup
make
```

http://localhost:1323/v2/users/stereosteve

> This will watch sql files + re-run `sqlc generate` + restart server when go files change.

## Schema dump

```
docker compose exec db bash
export discoveryDbUrl='a_db_url'
pg_dump $discoveryDbUrl --schema-only --no-owner --no-acl > /sql/schema1.sql
```

If you re-dump schema, reset dev postgres state:

```
docker compose down --volumes
docker compose up -d
```
