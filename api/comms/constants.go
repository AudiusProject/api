package comms

var (
	SigHeader             = "x-sig"
	SignatureTimeToLiveMs = int64(1000 * 60 * 60 * 12) // 12 hours
)

const (
	// Members in a group chat, creator included.
	MaxGroupChatMembers = 10

	// Group chat ids are "grp_" + random characters. Hashids never contain
	// "_", so a group id can't collide with a 1:1 "a:b" id.
	GroupChatIDPrefix = "grp_"
)
