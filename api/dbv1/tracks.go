package dbv1

import (
	"context"
	"encoding/json"
	"fmt"

	"api.audius.co/trashid"
)

type TracksParams struct {
	GetTracksParams
}

// IncludeID3TagsCtxKey is the request-context key used to opt in to ID3 tag
// query params on generated stream/preview URLs. The id3 middleware sets it
// from the `?id3=true` query param.
const IncludeID3TagsCtxKey = "includeID3Tags"

// Track is the standard track type containing all track data
type Track struct {
	GetTracksRow

	Permalink     string         `json:"permalink"`
	IsStreamable  bool           `json:"is_streamable"`
	Artwork       *SquareImage   `json:"artwork"`
	Stream        *MediaLink     `json:"stream"`
	Download      *MediaLink     `json:"download"`
	Preview       *MediaLink     `json:"preview"`
	UserID        trashid.HashId `json:"user_id"`
	User          User           `json:"user"`
	Collaborators []User         `json:"collaborators"`
	// PendingCollaborators is populated only on the requester's own tracks (so
	// the owner's edit form can preserve still-pending invites); empty otherwise.
	PendingCollaborators []User `json:"pending_collaborators"`
	Access               Access `json:"access"`

	FolloweeReposts    []*FolloweeRepost   `json:"followee_reposts"`
	FolloweeFavorites  []*FolloweeFavorite `json:"followee_favorites"`
	RemixOf            FullRemixOf         `json:"remix_of"`
	StreamConditions   *AccessGate         `json:"stream_conditions"`
	DownloadConditions *AccessGate         `json:"download_conditions"`
}

func (q *Queries) TracksKeyed(ctx context.Context, arg TracksParams) (map[int32]Track, error) {
	rawTracks, err := q.GetTracks(ctx, arg.GetTracksParams)
	if err != nil {
		return nil, err
	}

	userIds := []int32{}
	trackIds := make([]int32, 0, len(rawTracks))
	ownerByTrack := make(map[int32]int32, len(rawTracks))
	collectSplitUserIds := func(usage *AccessGate) {
		if usage == nil || usage.UsdcPurchase == nil {
			return
		}
		for _, split := range usage.UsdcPurchase.Splits {
			userIds = append(userIds, split.UserID)
		}
	}

	for _, rawTrack := range rawTracks {
		userIds = append(userIds, rawTrack.UserID)
		trackIds = append(trackIds, rawTrack.TrackID)
		ownerByTrack[rawTrack.TrackID] = rawTrack.UserID

		var remixOf RemixOf
		json.Unmarshal(rawTrack.RemixOf, &remixOf)
		for _, r := range remixOf.Tracks {
			userIds = append(userIds, r.ParentUserId)
		}

		collectSplitUserIds(rawTrack.StreamConditions)
		collectSplitUserIds(rawTrack.DownloadConditions)
	}

	// Fetch accepted + pending collaborators for these tracks in one query, and
	// fold their user IDs into the bulk user fetch below so each is fully
	// resolved. Accepted are embedded on every response; pending are embedded
	// only on the requester's own tracks (for the owner's edit form), so their
	// user IDs are only resolved for owned tracks.
	myID := arg.MyID.(int32)
	ownedTracks := map[int32]bool{}
	for _, rawTrack := range rawTracks {
		if rawTrack.UserID == myID {
			ownedTracks[rawTrack.TrackID] = true
		}
	}
	collaboratorRows, err := q.GetTrackCollaborators(ctx, trackIds)
	if err != nil {
		return nil, err
	}
	collaboratorsByTrack := map[int32][]int32{}
	pendingByTrack := map[int32][]int32{}
	for _, cr := range collaboratorRows {
		switch cr.Status {
		case "accepted":
			collaboratorsByTrack[cr.TrackID] = append(collaboratorsByTrack[cr.TrackID], cr.CollaboratorUserID)
			userIds = append(userIds, cr.CollaboratorUserID)
		case "pending":
			if ownedTracks[cr.TrackID] {
				pendingByTrack[cr.TrackID] = append(pendingByTrack[cr.TrackID], cr.CollaboratorUserID)
				userIds = append(userIds, cr.CollaboratorUserID)
			}
		}
	}

	userMap, err := q.UsersKeyed(ctx, GetUsersParams{
		MyID: arg.MyID.(int32),
		Ids:  userIds,
	})
	if err != nil {
		return nil, err
	}

	// Convert rawTracks to pointers
	trackPtrs := make([]*GetTracksRow, len(rawTracks))
	for i := range rawTracks {
		trackPtrs[i] = &rawTracks[i]
	}

	// Convert userMap to pointers
	userPtrMap := make(map[int32]*User)
	for id, user := range userMap {
		userCopy := user // Create a copy to avoid modifying the original
		userPtrMap[id] = &userCopy
	}

	// Read solana wallet from context (set by middleware) for token gate checks
	solanaWallet, _ := ctx.Value("solanaWallet").(string)
	accessMap, err := q.GetBulkTrackAccess(ctx, arg.MyID.(int32), trackPtrs, userPtrMap, solanaWallet)
	if err != nil {
		return nil, err
	}

	trackMap := map[int32]Track{}
	for _, rawTrack := range rawTracks {
		rawTrack.ID, _ = trashid.EncodeHashId(int(rawTrack.TrackID))
		user, ok := userMap[rawTrack.UserID]
		if !ok {
			continue
		}

		if rawTrack.FieldVisibility == nil || string(rawTrack.FieldVisibility) == "null" {
			rawTrack.FieldVisibility = []byte(`{
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
		json.Unmarshal(rawTrack.RemixOf, &remixOf)
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

		// Resolve accepted collaborators (order preserved from the query).
		collaborators := []User{}
		for _, cid := range uniqueCollaboratorIDs(collaboratorsByTrack[rawTrack.TrackID], ownerByTrack[rawTrack.TrackID]) {
			if cu, ok := userMap[cid]; ok {
				collaborators = append(collaborators, cu)
			}
		}

		// Resolve pending collaborators (only present for the owner's own tracks).
		pendingCollaborators := []User{}
		for _, cid := range uniqueCollaboratorIDs(pendingByTrack[rawTrack.TrackID], ownerByTrack[rawTrack.TrackID]) {
			if cu, ok := userMap[cid]; ok {
				pendingCollaborators = append(pendingCollaborators, cu)
			}
		}

		// Get access from the bulk access map
		access := accessMap[rawTrack.TrackID]

		var id3Tags *Id3Tags
		if includeID3Tags, _ := ctx.Value(IncludeID3TagsCtxKey).(bool); includeID3Tags {
			id3Tags = &Id3Tags{
				Title:  rawTrack.Title.String,
				Artist: user.Name.String,
			}
		}

		// A track is streamable unless it was deleted or its owner is no longer
		// active - either the artist deactivated their own account or the
		// account was delisted by the trusted notifier.
		isStreamable := !rawTrack.IsDelete && !user.IsDeactivated

		// Two reasons to leave a media link nil, both with the same effect: the
		// URL should never be handed out, and the endpoints report the track as
		// unavailable instead.
		//
		// A track row can have empty cid columns (e.g. an upload-v2 row whose
		// track_cid/orig_file_cid backfill never ran), and signing an empty cid
		// produces a content-node URL that is guaranteed to 404.
		//
		// A non-streamable track is worse: the cid is real, so the signed URL
		// works. The stream and download endpoints reject these, but that only
		// closes those two routes - anyone reading the track response could
		// still fetch the audio straight from the content node. Preview is
		// included because a preview clip is still the artist's audio.
		var stream *MediaLink
		if isStreamable && access.Stream && rawTrack.TrackCid.String != "" {
			stream, err = mediaLink(rawTrack.TrackCid.String, rawTrack.TrackID, arg.MyID.(int32), id3Tags)
			if err != nil {
				return nil, err
			}
		}

		var download *MediaLink
		if isStreamable && rawTrack.IsDownloadable && access.Download {
			if cid := rawTrack.DownloadCid(); cid != "" {
				download, err = mediaLink(cid, rawTrack.TrackID, arg.MyID.(int32), nil)
				if err != nil {
					return nil, err
				}
			}
		}

		var preview *MediaLink
		if isStreamable && rawTrack.PreviewCid.String != "" {
			preview, err = mediaLink(rawTrack.PreviewCid.String, rawTrack.TrackID, arg.MyID.(int32), id3Tags)
			if err != nil {
				return nil, err
			}
		}

		track := Track{
			GetTracksRow:         rawTrack,
			IsStreamable:         isStreamable,
			Permalink:            fmt.Sprintf("/%s/%s", user.Handle.String, rawTrack.Slug.String),
			Artwork:              squareImageStruct(rawTrack.CoverArtSizes, rawTrack.CoverArt),
			Stream:               stream,
			Download:             download,
			Preview:              preview,
			User:                 user,
			UserID:               user.ID,
			Collaborators:        collaborators,
			PendingCollaborators: pendingCollaborators,
			FolloweeFavorites:    fullFolloweeFavorites(rawTrack.FolloweeFavorites),
			FolloweeReposts:      fullFolloweeReposts(rawTrack.FolloweeReposts),
			RemixOf:              fullRemixOf,
			StreamConditions:     rawTrack.StreamConditions,
			DownloadConditions:   rawTrack.DownloadConditions,
			Access:               access,
		}
		trackMap[rawTrack.TrackID] = track
	}

	return trackMap, nil
}

func (q *Queries) Tracks(ctx context.Context, arg TracksParams) ([]Track, error) {
	trackMap, err := q.TracksKeyed(ctx, arg)
	if err != nil {
		return nil, err
	}

	// return in same order as input list of ids
	// some ids may be not found...
	tracks := []Track{}
	for _, id := range arg.Ids {
		if t, found := trackMap[id]; found {
			tracks = append(tracks, t)
		}
	}

	return tracks, nil
}

func uniqueCollaboratorIDs(collaboratorIDs []int32, ownerID int32) []int32 {
	if len(collaboratorIDs) == 0 {
		return nil
	}

	seen := map[int32]struct{}{ownerID: {}}
	uniqueIDs := make([]int32, 0, len(collaboratorIDs))
	for _, collaboratorID := range collaboratorIDs {
		if _, ok := seen[collaboratorID]; ok {
			continue
		}
		seen[collaboratorID] = struct{}{}
		uniqueIDs = append(uniqueIDs, collaboratorID)
	}
	return uniqueIDs
}
