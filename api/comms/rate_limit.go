package comms

// RateLimitConfig contains all rate limiting configuration
type RateLimitConfig struct {
	TimeframeHours             int
	MaxNumMessages             int
	MaxNumMessagesPerRecipient int
	MaxNumNewChats             int
	MaxMessagesPerRecipient1s  int
	MaxMessagesPerRecipient10s int
	MaxMessagesPerRecipient60s int
}

// DefaultRateLimitConfig provides default rate limiting values
var DefaultRateLimitConfig = RateLimitConfig{
	TimeframeHours:             24,
	MaxNumMessages:             2000,
	MaxNumMessagesPerRecipient: 1000,
	MaxNumNewChats:             100000,
	MaxMessagesPerRecipient1s:  10,
	MaxMessagesPerRecipient10s: 70,
	MaxMessagesPerRecipient60s: 300,
}
