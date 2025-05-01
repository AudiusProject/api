package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/rendezvous"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

type blobInspect struct {
	ContentType string `json:"ContentType"`
	Size        int64  `json:"Size"`
}

type inspectResponse struct {
	Size        int64  `json:"size"`
	ContentType string `json:"content_type"`
}

func inspectTrack(track dbv1.FullTrack, original bool) (*inspectResponse, error) {
	var cid string
	if original {
		cid = track.OrigFileCid.String
	} else {
		cid = track.TrackCid.String
	}

	first, rest := rendezvous.GlobalHasher.ReplicaSet3(cid)

	hosts := append([]string{first}, rest...)
	var info blobInspect
	var lastErr error

	for _, host := range hosts {
		client := &http.Client{}
		req, err := http.NewRequest("GET", fmt.Sprintf("%s/internal/blobs/info/%s", host, cid), nil)
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("host %s returned status %d", host, resp.StatusCode)
			continue
		}

		if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
			resp.Body.Close()
			lastErr = err
			continue
		}
		resp.Body.Close()

		return &inspectResponse{
			Size:        info.Size,
			ContentType: info.ContentType,
		}, nil
	}

	return nil, fmt.Errorf("failed to fetch blob info from any host: %v", lastErr)
}

func (app *ApiServer) v1TrackInspect(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	trackId := c.Locals("trackId").(int)
	original := c.Query("original") == "true"

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.GetTracksParams{
		MyID: myId,
		Ids:  []int32{int32(trackId)},
	})
	if err != nil {
		return err
	}

	if len(tracks) == 0 {
		return sendError(c, 404, "track not found")
	}

	track := tracks[0]
	info, err := inspectTrack(track, original)
	if err != nil {
		return sendError(c, 500, err.Error())
	}

	return c.JSON(map[string]any{
		"data": info,
	})
}

func (app *ApiServer) v1TracksInspect(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	ids := decodeIdList(c)
	original := c.Query("original") == "true"

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.GetTracksParams{
		MyID: myId,
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	if len(tracks) == 0 {
		return sendError(c, 404, "track not found")
	}

	infos := make([]*inspectResponse, len(tracks))
	g := &errgroup.Group{}

	for i, track := range tracks {
		idx, t := i, track // Create new variables for the goroutine
		g.Go(func() error {
			info, err := inspectTrack(t, original)
			if err != nil {
				infos[idx] = nil
				return err
			}
			infos[idx] = info
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return sendError(c, 500, err.Error())
	}

	return c.JSON(map[string]any{
		"data": infos,
	})
}
