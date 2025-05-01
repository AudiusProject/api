package dbv1

import (
	"context"
	"encoding/json"
	"fmt"

	"bridgerton.audius.co/config"
	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/errgroup"
)

type FullTracksParams GetTracksParams

type FullTrack struct {
	GetTracksRow

	ID           trashid.HashId `json:"id"`
	Permalink    string         `json:"permalink"`
	IsStreamable bool           `json:"is_streamable"`
	Artwork      *SquareImage   `json:"artwork"`
	Stream       *MediaLink     `json:"stream"`
	Download     *MediaLink     `json:"download"`
	Preview      *MediaLink     `json:"preview"`
	UserID       trashid.HashId `json:"user_id"`
	User         FullUser       `json:"user"`
	Access       Access         `json:"access"`

	FolloweeReposts    []*FolloweeRepost2   `json:"followee_reposts"`
	FolloweeFavorites  []*FolloweeFavorite2 `json:"followee_favorites"`
	RemixOf            FullRemixOf          `json:"remix_of"`
	StreamConditions   *FullAccessGate      `json:"stream_conditions"`
	DownloadConditions *FullAccessGate      `json:"download_conditions"`
}

func (q *Queries) FullTracksKeyed(ctx context.Context, arg GetTracksParams) (map[int32]FullTrack, error) {
	batch := q.NewTrackBatch(ctx, arg)
	err := batch.Load()
	if err != nil {
		return nil, err
	}
	return batch.ToMap(), nil
}

func (q *Queries) OldFullTracksKeyed(ctx context.Context, arg GetTracksParams) (map[int32]FullTrack, error) {
	rawTracks, err := q.GetTracks(ctx, GetTracksParams(arg))
	if err != nil {
		return nil, err
	}

	userIds := []int32{}
	collectSplitUserIds := func(usage *AccessGate) {
		if usage == nil || usage.UsdcPurchase == nil {
			return
		}
		for _, split := range usage.UsdcPurchase.Splits {
			userIds = append(userIds, split.UserID)
		}
	}

	for _, track := range rawTracks {
		userIds = append(userIds, track.UserID)

		var remixOf RemixOf
		json.Unmarshal(track.RemixOf, &remixOf)
		for _, r := range remixOf.Tracks {
			userIds = append(userIds, r.ParentUserId)
		}

		collectSplitUserIds(track.StreamConditions)
		collectSplitUserIds(track.DownloadConditions)
	}

	userMap, err := q.FullUsersKeyed(ctx, GetUsersParams{
		MyID: arg.MyID,
		Ids:  userIds,
	})
	if err != nil {
		return nil, err
	}

	trackMap := map[int32]FullTrack{}
	for _, track := range rawTracks {
		user, ok := userMap[track.UserID]
		if !ok {
			continue
		}

		// Collect media links
		// TODO(API-49): support self-access via grants
		// see https://github.com/AudiusProject/audius-protocol/blob/4bd9fe80d8cca519844596061505ad8737579019/packages/discovery-provider/src/queries/query_helpers.py#L905
		stream := mediaLink(track.TrackCid.String, track.TrackID, arg.MyID.(int32))
		var download *MediaLink
		if track.IsDownloadable {
			download = mediaLink(track.OrigFileCid.String, track.TrackID, arg.MyID.(int32))
		}
		var preview *MediaLink
		if track.PreviewCid.String != "" {
			preview = mediaLink(track.PreviewCid.String, track.TrackID, arg.MyID.(int32))
		}

		if track.FieldVisibility == nil || string(track.FieldVisibility) == "null" {
			track.FieldVisibility = []byte(`{
			"mood":null,
			"tags":null,
			"genre":null,
			"share":null,
			"play_count":null,
			"remixes":null
			}`)
		}

		var remixOf RemixOf
		var fullRemixOf FullRemixOf
		json.Unmarshal(track.RemixOf, &remixOf)
		fullRemixOf = FullRemixOf{
			Tracks: make([]FullRemixOfTrack, len(remixOf.Tracks)),
		}
		for idx, r := range remixOf.Tracks {
			trackId, _ := trashid.EncodeHashId(int(r.ParentTrackId))
			fullRemixOf.Tracks[idx] = FullRemixOfTrack{
				HasRemixAuthorReposted: r.HasRemixAuthorReposted,
				HasRemixAuthorSaved:    r.HasRemixAuthorSaved,
				ParentTrackId:          trackId,
				User:                   userMap[r.ParentUserId],
			}
		}

		// Use download conditions if available, otherwise use stream conditions
		var downloadConditions *AccessGate
		if track.DownloadConditions != nil {
			downloadConditions = track.DownloadConditions
		} else {
			downloadConditions = track.StreamConditions
		}
		downloadAccess := q.GetTrackAccess(ctx, arg.MyID.(int32), downloadConditions, &track, &user)
		// If you can download it, you can stream it
		streamAccess := downloadAccess || q.GetTrackAccess(ctx, arg.MyID.(int32), track.StreamConditions, &track, &user)
		access := Access{
			Download: downloadAccess,
			Stream:   streamAccess,
		}

		fullTrack := FullTrack{
			GetTracksRow: track,

			ID:           trashid.HashId(track.TrackID),
			IsStreamable: !track.IsDelete && !user.IsDeactivated,
			Permalink:    fmt.Sprintf("/%s/%s", user.Handle.String, track.Slug.String),
			Artwork:      squareImageStruct(track.CoverArtSizes, track.CoverArt),
			Stream:       stream,
			Download:     download,
			Preview:      preview,
			User:         user,
			UserID:       user.ID,
			// FolloweeFavorites:  fullFolloweeFavorites(track.FolloweeFavorites),
			// FolloweeReposts:    fullFolloweeReposts(track.FolloweeReposts),
			RemixOf:            fullRemixOf,
			StreamConditions:   track.StreamConditions.toFullAccessGate(config.Cfg, userMap),
			DownloadConditions: track.DownloadConditions.toFullAccessGate(config.Cfg, userMap),
			Access:             access,
		}

		trackMap[track.TrackID] = fullTrack
	}

	return trackMap, nil
}

func (q *Queries) FullTracks(ctx context.Context, arg GetTracksParams) ([]FullTrack, error) {
	trackMap, err := q.FullTracksKeyed(ctx, arg)
	if err != nil {
		return nil, err
	}

	// return in same order as input list of ids
	// some ids may be not found...
	fullTracks := []FullTrack{}
	for _, id := range arg.Ids {
		if t, found := trackMap[id]; found {
			fullTracks = append(fullTracks, t)
		}
	}

	return fullTracks, nil
}

type MinTrack struct {
	ID                       trashid.HashId `json:"id"`
	Title                    pgtype.Text    `json:"title"`
	User                     MinUser        `json:"user"`
	Artwork                  *SquareImage   `json:"artwork"`
	Duration                 pgtype.Int4    `json:"duration"`
	Description              pgtype.Text    `json:"description"`
	Genre                    pgtype.Text    `json:"genre"`
	TrackCid                 pgtype.Text    `json:"track_cid"`
	PreviewCid               pgtype.Text    `json:"preview_cid"`
	OrigFileCid              pgtype.Text    `json:"orig_file_cid"`
	OrigFilename             pgtype.Text    `json:"orig_filename"`
	IsOriginalAvailable      bool           `json:"is_original_available"`
	Mood                     pgtype.Text    `json:"mood"`
	ReleaseDate              interface{}    `json:"release_date"`
	RemixOf                  interface{}    `json:"remix_of"`
	RepostCount              int32          `json:"repost_count"`
	FavoriteCount            int32          `json:"favorite_count"`
	CommentCount             pgtype.Int4    `json:"comment_count"`
	Tags                     pgtype.Text    `json:"tags"`
	IsDownloadable           bool           `json:"is_downloadable"`
	PlayCount                pgtype.Int8    `json:"play_count"`
	PinnedCommentID          pgtype.Int4    `json:"pinned_comment_id"`
	PlaylistsContainingTrack []int32        `json:"playlists_containing_track"`
	AlbumBacklink            interface{}    `json:"album_backlink"`
	IsStreamable             bool           `json:"is_streamable"`
	Permalink                string         `json:"permalink"`
}

func ToMinTrack(fullTrack FullTrack) MinTrack {
	return MinTrack{
		ID:                       fullTrack.ID,
		Title:                    fullTrack.Title,
		User:                     ToMinUser(fullTrack.User),
		Artwork:                  fullTrack.Artwork,
		Duration:                 fullTrack.Duration,
		Description:              fullTrack.Description,
		Genre:                    fullTrack.Genre,
		TrackCid:                 fullTrack.TrackCid,
		PreviewCid:               fullTrack.PreviewCid,
		OrigFileCid:              fullTrack.OrigFileCid,
		OrigFilename:             fullTrack.OrigFilename,
		IsOriginalAvailable:      fullTrack.IsOriginalAvailable,
		Mood:                     fullTrack.Mood,
		ReleaseDate:              fullTrack.ReleaseDate,
		RemixOf:                  fullTrack.RemixOf,
		RepostCount:              fullTrack.RepostCount,
		FavoriteCount:            fullTrack.FavoriteCount,
		CommentCount:             fullTrack.CommentCount,
		Tags:                     fullTrack.Tags,
		IsDownloadable:           fullTrack.IsDownloadable,
		PlayCount:                fullTrack.PlayCount,
		PinnedCommentID:          fullTrack.PinnedCommentID,
		PlaylistsContainingTrack: fullTrack.PlaylistsContainingTrack,
		AlbumBacklink:            nil,
		IsStreamable:             !fullTrack.IsDelete && !fullTrack.User.IsDeactivated,
		Permalink:                fmt.Sprintf("/%s/%s", fullTrack.User.Handle.String, fullTrack.Slug.String),
	}
}

func ToMinTracks(fullTracks []FullTrack) []MinTrack {
	result := make([]MinTrack, len(fullTracks))
	for i, track := range fullTracks {
		result[i] = ToMinTrack(track)
	}
	return result
}

type TrackBatch struct {
	ctx      context.Context
	q        *Queries
	myId     int32
	trackIds []int32
	trackMap map[int32]*FullTrack
	userMap  map[int32]FullUser
}

func (q *Queries) NewTrackBatch(ctx context.Context, arg GetTracksParams) *TrackBatch {
	return &TrackBatch{
		ctx:      ctx,
		q:        q,
		myId:     arg.MyID.(int32),
		trackIds: arg.Ids,
		trackMap: map[int32]*FullTrack{},
	}
}

func (batch *TrackBatch) ToMap() map[int32]FullTrack {
	m := map[int32]FullTrack{}
	for id, track := range batch.trackMap {
		m[id] = *track
	}
	return m
}

func (batch *TrackBatch) Load() error {
	rawTracks, err := batch.q.GetTracks(batch.ctx, GetTracksParams{
		Ids:  batch.trackIds,
		MyID: batch.myId,
	})
	if err != nil {
		return err
	}
	for _, rt := range rawTracks {
		batch.trackMap[rt.TrackID] = &FullTrack{
			GetTracksRow: rt,
			ID:           trashid.HashId(rt.TrackID),
		}
	}
	return batch.Enhance()
}

func (batch *TrackBatch) Enhance() error {
	// load related stuff in parallel
	g, _ := errgroup.WithContext(batch.ctx)
	g.Go(batch.LoadUsers)
	g.Go(batch.LoadFolloweeActions)
	// todo: current user: is saved, is reposted
	// todo: remixes
	// todo: access
	// todo: permalink stuff

	err := g.Wait()
	if err != nil {
		return err
	}

	// finalize
	for _, track := range batch.trackMap {
		if track.FieldVisibility == nil || string(track.FieldVisibility) == "null" {
			track.FieldVisibility = []byte(`{
				"mood":null,
				"tags":null,
				"genre":null,
				"share":null,
				"play_count":null,
				"remixes":null
			}`)
		}

		// RemixOf:            fullRemixOf,
		// Access:             access,

		// downloadAccess := q.GetTrackAccess(ctx, arg.MyID.(int32), downloadConditions, &track, &user)
		// // If you can download it, you can stream it
		// streamAccess := downloadAccess || q.GetTrackAccess(ctx, arg.MyID.(int32), track.StreamConditions, &track, &user)
		// access := Access{
		// 	Download: downloadAccess,
		// 	Stream:   streamAccess,
		// }

		// Collect media links
		// TODO(API-49): support self-access via grants
		// see https://github.com/AudiusProject/audius-protocol/blob/4bd9fe80d8cca519844596061505ad8737579019/packages/discovery-provider/src/queries/query_helpers.py#L905
		track.Stream = mediaLink(track.TrackCid.String, track.TrackID, batch.myId)
		if track.IsDownloadable {
			track.Download = mediaLink(track.OrigFileCid.String, track.TrackID, batch.myId)
		} else {
			track.Download = track.Stream
		}
		if track.PreviewCid.String != "" {
			track.Preview = mediaLink(track.PreviewCid.String, track.TrackID, batch.myId)
		}

		track.IsStreamable = !track.IsDelete && !track.User.IsDeactivated
		track.Permalink = fmt.Sprintf("/%s/%s", track.User.Handle.String, track.Slug.String)
		track.Artwork = squareImageStruct(track.CoverArtSizes, track.CoverArt)
		track.UserID = track.User.ID
		track.StreamConditions = track.GetTracksRow.StreamConditions.toFullAccessGate(config.Cfg, batch.userMap)
		track.DownloadConditions = track.GetTracksRow.DownloadConditions.toFullAccessGate(config.Cfg, batch.userMap)
	}

	return nil
}

func (batch *TrackBatch) LoadUsers() error {
	userIds := make([]int32, 0, len(batch.trackIds))
	for _, track := range batch.trackMap {
		userIds = append(userIds, track.GetTracksRow.UserID)
	}
	userMap, err := batch.q.FullUsersKeyed(batch.ctx, GetUsersParams{
		MyID: batch.myId,
		Ids:  userIds,
	})

	batch.userMap = userMap
	for _, track := range batch.trackMap {
		track.User = userMap[int32(track.GetTracksRow.UserID)]
	}
	return err
}

func (batch *TrackBatch) LoadFolloweeActions() error {
	// load up followee_saves + followee_reposts
	actions, err := batch.q.FolloweeActions(batch.ctx, FolloweeActionsParams{
		Ids:  batch.trackIds,
		MyID: batch.myId,
	})
	for _, action := range actions {
		track := batch.trackMap[action.ItemID]
		switch action.Verb {
		case "save":
			track.FolloweeFavorites = append(track.FolloweeFavorites, &FolloweeFavorite2{
				FavoriteItemId: trashid.HashId(action.ItemID),
				FavoriteType:   "SaveType.track",
				UserId:         trashid.HashId(action.UserID),
				CreatedAt:      action.CreatedAt,
			})
		case "repost":
			track.FolloweeReposts = append(track.FolloweeReposts, &FolloweeRepost2{
				RepostItemId: trashid.HashId(action.ItemID),
				RepostType:   "RepostType.track",
				UserId:       trashid.HashId(action.UserID),
				CreatedAt:    action.CreatedAt,
			})
		default:
			msg := fmt.Sprintf("Unknown verb: %s", action.Verb)
			panic(msg)
		}
	}
	return err
}
