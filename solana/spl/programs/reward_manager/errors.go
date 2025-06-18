package reward_manager

// Custom errors returned by the RewardManager program.
//
// Caution: Some error values overlap with system program errors.
//
// See also: https://github.com/AudiusProject/audius-protocol/blob/2a37bcff1bb1a82efdf187d1723b3457dc0dcb9b/solana-programs/reward-manager/program/src/error.rs
type RewardManagerError int

const (
	IncorrectOwner RewardManagerError = iota
	SignCollision
	WrongSigner
	NotEnoughSigners
	Secp256InstructionMissing
	InstructionLoadError
	RepeatedSenders
	SignatureVerificationFailed
	OperatorCollision
	AlreadySent
	IncorrectMessages
	MessagesOverflow
	MathOverflow
	InvalidRecipient
)

func (e RewardManagerError) String() string {
	switch e {
	case IncorrectOwner:
		return "IncorrectOwner"
	case SignCollision:
		return "SignCollision"
	case WrongSigner:
		return "WrongSigner"
	case NotEnoughSigners:
		return "NotEnoughSigners"
	case Secp256InstructionMissing:
		return "Secp256InstructionMissing"
	case InstructionLoadError:
		return "InstructionLoadError"
	case RepeatedSenders:
		return "RepeatedSenders"
	case SignatureVerificationFailed:
		return "SignatureVerificationFailed"
	case OperatorCollision:
		return "OperatorCollision"
	case AlreadySent:
		return "AlreadySent"
	case IncorrectMessages:
		return "IncorrectMessages"
	case MessagesOverflow:
		return "MessagesOverflow"
	case MathOverflow:
		return "MathOverflow"
	case InvalidRecipient:
		return "InvalidRecipient"
	default:
		return "UnknownError"
	}
}
