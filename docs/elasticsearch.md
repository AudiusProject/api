# Elasticsearch

When making search changes, it's nice to test with client + prod data.

You can reindex all the prod data to local machine thusly:

- in `.env` add `elasticsearchUrl=http://localhost:21400`
- `time go run main.go es-indexer drop all`
- `go run main.go`

Over in `audius-protocol`:

- in `packages/sdk/src/sdk/config/production.ts` set `"apiEndpoint": "http://localhost:1323",`
- `npm run preview:prod`

To debug scoring math:

- in `dsl_helpers.go` set `explain := true` (but don't commit that)
- in server console you'll see scoring details printed out.

## See the full DSL:

Adding `debug=true` query param will add response headers with the full DSL (`x-track-dsl`, `x-user-dsl`, etc.)

```
http://localhost:1323/v1/full/search/full?includePurchaseable=true&kind=tracks&limit=12&offset=0&query=sun&sort_method=recent&user_id=aNzoj&api_key=8acf5eb7436ea403ee536a7334faa5e9ada4b50f&app_name=audius-client&debug=true
```

You can also get the mapping from the local elasticsearch instance:

```
http://localhost:21400/users
```

Providing the mapping and query (and possibly scoring output from above) to an LLM can be a good way to ask for suggestions for adjusting the query.
