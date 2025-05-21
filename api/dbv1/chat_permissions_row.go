package dbv1

import (
	"encoding/json"
	"strings"

	"bridgerton.audius.co/trashid"
)

type ChatPermissionsRow struct {
	UserID                   trashid.HashId `db:"user_id" json:"user_id"`
	Permits                  string         `db:"permits" json:"permits"`
	CurrentUserHasPermission bool           `db:"current_user_has_permission" json:"current_user_has_permission"`
}

func (row ChatPermissionsRow) MarshalJSON() ([]byte, error) {
	type Alias ChatPermissionsRow
	permitList := strings.Split(row.Permits, ",")
	permits := row.Permits
	if len(permitList) > 0 {
		permits = permitList[0]
	}

	return json.Marshal(&struct {
		Alias
		Permits    string   `json:"permits"`
		PermitList []string `json:"permit_list"`
	}{
		Alias:      Alias(row),
		Permits:    permits,
		PermitList: permitList,
	})
}
