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
	// Action_SubmitToContest entries a track in an open-contest event.
	// EntityType is Event, EntityId is the contest event_id, and the
	// submitted track_id rides on the metadata JSON ({"track_id": <id>}).
	Action_SubmitToContest = "SubmitToContest"
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
