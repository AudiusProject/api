package dbv1

import (
	"context"
)

type FullManagerGrant struct {
	GetGrantsForUserIdRow
	GranteeUserID *struct{} `json:"grantee_user_id,omitempty"`
}

type FullManagedUserGrant struct {
	GetGrantsForGranteeUserIdRow
	GranteeUserID *struct{} `json:"grantee_user_id,omitempty"`
}

type FullManager struct {
	Manager FullUser         `json:"manager"`
	Grant   FullManagerGrant `json:"grant"`
}

type FullManagedUser struct {
	User  FullUser             `json:"user"`
	Grant FullManagedUserGrant `json:"grant"`
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
			Grant:   FullManagerGrant{GetGrantsForUserIdRow: grant},
		}
	}

	return managers, nil
}

func (q *Queries) FullManagedUsers(ctx context.Context, params GetGrantsForGranteeUserIdParams) ([]FullManagedUser, error) {
	grants, err := q.GetGrantsForGranteeUserId(ctx, GetGrantsForGranteeUserIdParams{
		IsRevoked:  params.IsRevoked,
		IsApproved: params.IsApproved,
		UserID:     params.UserID,
	})
	if err != nil {
		return nil, err
	}

	user_ids := make([]int32, len(grants))
	for i, grant := range grants {
		user_ids[i] = int32(grant.UserID)
	}

	users, err := q.FullUsersKeyed(ctx, GetUsersParams{
		Ids:  user_ids,
		MyID: params.UserID,
	})

	if err != nil {
		return nil, err
	}

	managedUsers := make([]FullManagedUser, len(grants))
	for i, grant := range grants {
		managedUsers[i] = FullManagedUser{
			User:  users[int32(grant.UserID)],
			Grant: FullManagedUserGrant{GetGrantsForGranteeUserIdRow: grant},
		}
	}

	return managedUsers, nil
}
