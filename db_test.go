package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

func TestDB(t *testing.T) {

	urlExample := "postgres://postgres:example@localhost:21300/postgres"
	conn, err := pgx.Connect(context.Background(), urlExample)
	assert.NoError(t, err)
	defer conn.Close(context.Background())

	var name string
	var weight int64
	err = conn.QueryRow(context.Background(), "select 'bridge', 2").Scan(&name, &weight)
	assert.NoError(t, err)

	assert.Equal(t, "bridge", name)
	assert.EqualValues(t, 2, weight)
	fmt.Println(name, weight)
}
