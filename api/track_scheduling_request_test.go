package api

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

// Track write handlers decode the request then re-encode it as chain metadata.
// Both true and false must survive; omission must leave existing state alone.
func TestTrackSchedulingRequestMetadata(t *testing.T) {
	for _, operation := range []string{"create", "update"} {
		for _, flag := range []string{"true", "false", "omitted"} {
			t.Run(operation+"/"+flag, func(t *testing.T) {
				input := map[string]any{"release_date": "2026-10-20T07:00:00.000Z"}
				if flag != "omitted" {
					input["is_scheduled_release"] = flag == "true"
				}
				body, err := json.Marshal(input)
				require.NoError(t, err)
				var request any = &CreateTrackRequest{}
				if operation == "update" {
					request = &UpdateTrackRequest{}
				}
				require.NoError(t, json.Unmarshal(body, request))
				metadataBytes, err := json.Marshal(request)
				require.NoError(t, err)
				var metadata map[string]any
				require.NoError(t, json.Unmarshal(metadataBytes, &metadata))
				require.Equal(t, input["release_date"], metadata["release_date"])
				if flag == "omitted" {
					require.NotContains(t, metadata, "is_scheduled_release")
				} else {
					require.Equal(t, input["is_scheduled_release"], metadata["is_scheduled_release"])
				}
			})
		}
	}
}
