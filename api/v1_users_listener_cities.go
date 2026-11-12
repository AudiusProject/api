package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersListenerCitiesQueryParams struct {
	Time  string `query:"time" default:"month" validate:"oneof=week month all"`
	Limit int    `query:"limit" default:"10" validate:"min=1,max=50"`
}

type ListenerCity struct {
	City      string `json:"city" db:"city"`
	Region    string `json:"region" db:"region"`
	Country   string `json:"country" db:"country"`
	PlayCount int64  `json:"play_count" db:"play_count"`
}

// v1UsersListenerCities returns the cities that played the artist's tracks
// the most, from aggregate_daily_city_plays (see jobs.CityPlaysJob). The
// artist's own plays and plays with no city are not counted.
func (app *ApiServer) v1UsersListenerCities(c *fiber.Ctx) error {
	params := GetUsersListenerCitiesQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	// Days are UTC; "week" is today plus the 6 days before it.
	var since *time.Time
	today := time.Now().UTC().Truncate(24 * time.Hour)
	switch params.Time {
	case "week":
		t := today.AddDate(0, 0, -6)
		since = &t
	case "month":
		t := today.AddDate(0, 0, -29)
		since = &t
	}

	rows, err := app.pool.Query(c.Context(), `
		SELECT a.city, a.region, a.country, SUM(a.count)::bigint AS play_count
		FROM tracks t
		JOIN aggregate_daily_city_plays a ON a.play_item_id = t.track_id
		WHERE t.owner_id = @userId
			AND t.is_current
			AND (@since::date IS NULL OR a.day >= @since::date)
		GROUP BY a.city, a.region, a.country
		ORDER BY play_count DESC, a.city, a.region, a.country
		LIMIT @limit
	`, pgx.NamedArgs{
		"userId": app.getUserId(c),
		"since":  since,
		"limit":  params.Limit,
	})
	if err != nil {
		return err
	}
	cities, err := pgx.CollectRows(rows, pgx.RowToStructByName[ListenerCity])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": cities,
	})
}
