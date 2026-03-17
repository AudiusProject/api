package api

import (
	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

// TrackDownloadCount is the response shape for a single track's download count.
type TrackDownloadCount struct {
	ID            string `json:"id"`
	DownloadCount int64  `json:"download_count"`
}

func (app *ApiServer) v1TracksDownloadCounts(c *fiber.Ctx) error {
	ids := decodeIdList(c)
	if len(ids) == 0 {
		return c.JSON(fiber.Map{"data": []TrackDownloadCount{}})
	}

	rows, err := app.queries.GetTrackDownloadCounts(c.Context(), ids)
	if err != nil {
		return err
	}

	result := make([]TrackDownloadCount, 0, len(rows))
	for _, row := range rows {
		id, _ := trashid.EncodeHashId(int(row.TrackID))
		result = append(result, TrackDownloadCount{
			ID:            id,
			DownloadCount: row.DownloadCount,
		})
	}

	return c.JSON(fiber.Map{"data": result})
}

func (app *ApiServer) v1TrackDownloadCount(c *fiber.Ctx) error {
	trackId := c.Locals("trackId").(int)

	rows, err := app.queries.GetTrackDownloadCounts(c.Context(), []int32{int32(trackId)})
	if err != nil {
		return err
	}

	var count int64
	id, _ := trashid.EncodeHashId(trackId)
	if len(rows) > 0 {
		count = rows[0].DownloadCount
	}

	return c.JSON(fiber.Map{
		"data": TrackDownloadCount{
			ID:            id,
			DownloadCount: count,
		},
	})
}
