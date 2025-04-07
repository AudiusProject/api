package dbv1

import (
	"context"

	"bridgerton.audius.co/trashid"
)

type FullDeveloperApp struct {
	GetDeveloperAppsRow

	UserID string `json:"user_id"`
}

func (q *Queries) FullDeveloperApps(ctx context.Context, arg GetDeveloperAppsParams) ([]FullDeveloperApp, error) {
	rawDeveloperApps, err := q.GetDeveloperApps(ctx, arg)
	if err != nil {
		return nil, err
	}

	fullDeveloperApps := make([]FullDeveloperApp, 0, len(rawDeveloperApps))
	for _, d := range rawDeveloperApps {
		id, _ := trashid.EncodeHashId(int(d.UserID.Int32))
		fullDeveloperApps = append(fullDeveloperApps, FullDeveloperApp{
			GetDeveloperAppsRow: d,
			UserID:              id,
		})
	}

	return fullDeveloperApps, nil
}

type MinDeveloperApp struct {
	FullDeveloperApp
}

func ToMinDeveloperApp(fullDeveloperApp FullDeveloperApp) MinDeveloperApp {
	return MinDeveloperApp{
		FullDeveloperApp: fullDeveloperApp,
	}
}

func ToMinDeveloperApps(fullDeveloperApps []FullDeveloperApp) []MinDeveloperApp {
	result := make([]MinDeveloperApp, len(fullDeveloperApps))
	for i, d := range fullDeveloperApps {
		result[i] = ToMinDeveloperApp(d)
	}
	return result
}
