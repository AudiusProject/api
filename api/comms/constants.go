package comms

var (
	SigHeader             = "x-sig"
	SignatureTimeToLiveMs = int64(1000 * 60 * 60 * 12) // 12 hours

	// TODO: Do we need these configurable?
	// Rate limit config
	RateLimitRulesBucketName            = "rateLimitRules"
	RateLimitTimeframeHours             = "timeframeHours"
	RateLimitMaxNumMessages             = "maxNumMessages"
	RateLimitMaxNumMessagesPerRecipient = "maxNumMessagesPerRecipient"
	RateLimitMaxNumNewChats             = "maxNumNewChats"

	DefaultRateLimitRules = map[string]int{
		RateLimitTimeframeHours:             24,
		RateLimitMaxNumMessages:             2000,
		RateLimitMaxNumMessagesPerRecipient: 1000,
		RateLimitMaxNumNewChats:             100000,
	}
)
