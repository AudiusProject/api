package dbv1

import (
	"context"

	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type GetDeveloperAppsParams struct {
	UserID  int    `json:"user_id"`
	Address string `json:"address"`
}

type GetDeveloperAppsRow struct {
	Address     string         `json:"address"`
	UserID      trashid.HashId `json:"user_id"`
	Name        string         `json:"name"`
	Description pgtype.Text    `json:"description"`
	ImageUrl    pgtype.Text    `json:"image_url"`
}

func (q *Queries) GetDeveloperApps(ctx context.Context, arg GetDeveloperAppsParams) ([]GetDeveloperAppsRow, error) {
	rows, err := q.db.Query(ctx, mustGetQuery("get_developer_apps.sql"), toNamedArgs(arg))
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByNameLax[GetDeveloperAppsRow])
}
