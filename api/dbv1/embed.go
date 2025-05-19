package dbv1

import (
	"embed"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

//go:embed queries_sqlc_eject/*
var embeddedQueries embed.FS

func mustGetQuery(fileName string) string {
	b, err := embeddedQueries.ReadFile("queries_sqlc_eject/" + fileName)
	if err != nil {
		panic("MustGetQuery failed: " + fileName)
	}
	return string(b)
}

// converts a struct (with json tags) to a named arg map
// by round tripping thru json.
// This makes it easy to transition from sqlc generated argument structs.
func toNamedArgs(arg any) pgx.StrictNamedArgs {
	data, err := json.Marshal(arg)
	if err != nil {
		panic(err)
	}

	var m pgx.StrictNamedArgs
	err = json.Unmarshal(data, &m)
	if err != nil {
		panic(err)
	}
	return m
}
