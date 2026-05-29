package indexer

const (
	Action_Create       = "Create"
	Action_Update       = "Update"
	Action_Delete       = "Delete"
	Action_Save         = "Save"
	Action_Unsave       = "Unsave"
	Action_Repost       = "Repost"
	Action_Unrepost     = "Unrepost"
	Action_Download     = "Download"
	Action_Share        = "Share"
	Action_Follow       = "Follow"
	Action_Unfollow     = "Unfollow"
	Action_Subscribe    = "Subscribe"
	Action_Unsubscribe  = "Unsubscribe"
	Action_Verify       = "Verify"
	Action_View         = "View"
	Action_ViewPlaylist = "ViewPlaylist"
	Action_Approve      = "Approve"
	Action_Reject       = "Reject"
	Action_Pin          = "Pin"
	Action_Unpin        = "Unpin"
	Action_Mute         = "Mute"
	Action_Unmute       = "Unmute"
	Action_React        = "React"
	Action_Unreact      = "Unreact"
	Action_Report       = "Report"
)

const (
	Entity_Track            = "Track"
	Entity_User             = "User"
	Entity_Playlist         = "Playlist"
	Entity_Comment          = "Comment"
	Entity_Notification     = "Notification"
	Entity_AssociatedWallet = "AssociatedWallet"
	Entity_Grant            = "Grant"
	Entity_DeveloperApp     = "DeveloperApp"
	Entity_Event            = "Event"
)

// Track field constraints applied at EM-event indexing time. Track genres are
// arbitrary user-supplied strings (not an enum); enforce only a length cap.
// Mirrors discovery-provider's `CHARACTER_LIMIT_GENRE` so that when the Go
// indexer takes over track-entity event consumption from the Python
// discovery-provider, the rule stays consistent.
const (
	MaxTrackGenreLength = 100
)
