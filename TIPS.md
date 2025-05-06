Quickly scan some sql into a map and respond:

```go
	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id": app.getUserId(c),
	})
	if err != nil {
		return err
	}

	stuff, err := pgx.CollectRows(rows, pgx.RowToMap)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": stuff,
	})
```
