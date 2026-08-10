package dbv1

import (
	"encoding/json"
	"sort"
	"testing"
)

// The shape below is what the indexer has always written and what production
// rows contain. Reading it wrongly does not error anywhere a user can see —
// it silently denies access to tracks people paid for — so it is pinned here.
func TestParseTrackRemovals(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want []trackRemoval
	}{
		{
			name: "production shape",
			raw:  `{"1284768821": {"time": 1725873897}}`,
			want: []trackRemoval{{TrackID: 7, PlaylistID: 1284768821, RemovalTime: 1725873897}},
		},
		{
			name: "several albums, each with its own removal time",
			raw:  `{"100": {"time": 111}, "200": {"time": 222}}`,
			want: []trackRemoval{
				{TrackID: 7, PlaylistID: 100, RemovalTime: 111},
				{TrackID: 7, PlaylistID: 200, RemovalTime: 222},
			},
		},
		{
			name: "empty object — the column default",
			raw:  `{}`,
			want: []trackRemoval{},
		},
		{
			name: "absent",
			raw:  ``,
			want: nil,
		},
		// Anything we cannot read must deny rather than grant.
		{
			name: "array shape is not what the indexer writes",
			raw:  `[{"playlist_id": 100, "removal_time": "111"}]`,
			want: nil,
		},
		{
			name: "non-numeric playlist key is skipped",
			raw:  `{"not-an-id": {"time": 111}}`,
			want: []trackRemoval{},
		},
		{
			name: "garbage",
			raw:  `{`,
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTrackRemovals(7, json.RawMessage(tt.raw))
			sort.Slice(got, func(i, j int) bool { return got[i].PlaylistID < got[j].PlaylistID })

			if tt.want == nil && got != nil {
				t.Fatalf("got %+v, want nil", got)
			}
			if len(got) != len(tt.want) {
				t.Fatalf("got %d removals %+v, want %d", len(got), got, len(tt.want))
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("[%d] = %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// The parsed removals are handed to Postgres as a jsonb array and read back
// with jsonb_to_recordset(... AS r(track_id int, playlist_id int,
// removal_time bigint)). The struct tags are that column contract, so a rename
// here has to be a rename there.
func TestTrackRemovalMarshalsToRecordsetColumns(t *testing.T) {
	b, err := json.Marshal([]trackRemoval{{TrackID: 7, PlaylistID: 100, RemovalTime: 111}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	const want = `[{"track_id":7,"playlist_id":100,"removal_time":111}]`
	if string(b) != want {
		t.Errorf("got %s, want %s", b, want)
	}
}
