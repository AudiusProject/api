package dbv1

import (
	"context"
)

type FullGrant struct {
	GetGrantsForUserIdRow
	GranteeUserID *struct{} `json:"grantee_user_id,omitempty"`
}

type FullManager struct {
	Manager FullUser  `json:"manager"`
	Grant   FullGrant `json:"grant"`
}

func (q *Queries) FullManagers(ctx context.Context, params GetGrantsForUserIdParams) ([]FullManager, error) {

	grants, err := q.GetGrantsForUserId(ctx, params)
	if err != nil {
		return nil, err
	}

	user_ids := make([]int32, len(grants))
	for i, grant := range grants {
		user_ids[i] = int32(grant.GranteeUserID)
	}

	users, err := q.FullUsersKeyed(ctx, GetUsersParams{
		Ids:  user_ids,
		MyID: params.UserID,
	})

	if err != nil {
		return nil, err
	}

	managers := make([]FullManager, len(grants))
	for i, grant := range grants {
		managers[i] = FullManager{
			Manager: users[int32(grant.GranteeUserID)],
			Grant:   FullGrant{GetGrantsForUserIdRow: grant},
		}
	}

	return managers, nil
}
