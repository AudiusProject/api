package dbv1

import (
	"context"
	"time"

	"api.audius.co/trashid"
)

// FullTrackCollaboratorInvite is a collaborator credit from the invited user's
// perspective: which track, who invited them, and the current status.
type FullTrackCollaboratorInvite struct {
	TrackID   string    `json:"track_id"`
	Status    string    `json:"status"`
	InvitedBy User      `json:"invited_by"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FullTrackCollaboratorInvites resolves a user's collaborator invites, embedding
// the inviter as a full user object (mirrors FullManagers).
func (q *Queries) FullTrackCollaboratorInvites(ctx context.Context, params GetTrackCollaboratorInvitesForUserParams) ([]FullTrackCollaboratorInvite, error) {
	rows, err := q.GetTrackCollaboratorInvitesForUser(ctx, params)
	if err != nil {
		return nil, err
	}

	inviterIds := make([]int32, len(rows))
	for i, row := range rows {
		inviterIds[i] = row.InvitedBy
	}

	users, err := q.UsersKeyed(ctx, GetUsersParams{
		Ids:  inviterIds,
		MyID: params.UserID,
	})
	if err != nil {
		return nil, err
	}

	invites := make([]FullTrackCollaboratorInvite, len(rows))
	for i, row := range rows {
		trackID, _ := trashid.EncodeHashId(int(row.TrackID))
		invites[i] = FullTrackCollaboratorInvite{
			TrackID:   trackID,
			Status:    row.Status,
			InvitedBy: users[row.InvitedBy],
			CreatedAt: row.CreatedAt,
			UpdatedAt: row.UpdatedAt,
		}
	}

	return invites, nil
}
