package comms

import (
	"fmt"

	"bridgerton.audius.co/trashid"
)

// ChatID return a encodedUser1:encodedUser2 ID where encodedUser1 is < encodedUser2
// which is the convention used to make chat IDs deterministic.
// See makeChatId in SDK: packages/common/src/store/pages/chat/utils.ts
func ChatID(id1, id2 int) string {
	// TODO: Handle errors
	user1IdEncoded, _ := trashid.EncodeHashId(id1)
	user2IdEncoded, _ := trashid.EncodeHashId(id2)
	chatId := fmt.Sprintf("%s:%s", user1IdEncoded, user2IdEncoded)
	if user2IdEncoded < user1IdEncoded {
		chatId = fmt.Sprintf("%s:%s", user2IdEncoded, user1IdEncoded)
	}
	return chatId
}

// Returns a unique Message ID for a blast message in a chat.
func BlastMessageID(blastID, chatID string) string {
	return blastID + chatID
}
