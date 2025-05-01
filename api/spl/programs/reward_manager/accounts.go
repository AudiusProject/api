package reward_manager

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/AudiusProject/audiusd/pkg/rewards"
	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type RewardManagerState struct {
	Version      uint8
	TokenAccount solana.PublicKey
	Manager      solana.PublicKey
	MinVotes     uint8
}

type Attestation struct {
	SenderEthAddress   string
	Claim              rewards.RewardClaim
	OperatorEthAddress string
}

func (data *Attestation) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	senderBytes, err := decoder.ReadNBytes(20)
	if err != nil {
		return err
	}
	data.SenderEthAddress = "0x" + hex.EncodeToString(senderBytes)

	recipientBytes, err := decoder.ReadNBytes(20)
	if err != nil {
		return err
	}
	data.Claim.RecipientEthAddress = "0x" + hex.EncodeToString(recipientBytes)

	// Read separator _
	decoder.SkipBytes(uint(1))

	amount, err := decoder.ReadUint64(binary.LittleEndian)
	if err != nil {
		return err
	}
	data.Claim.Amount = amount

	// Read separator _
	decoder.SkipBytes(uint(1))

	disbursementIdBytes := []byte{}
	bytesLeft := 32
	for {
		b, err := decoder.ReadByte()
		if err != nil {
			return err
		}

		if b == 0 || b == []byte("_")[0] {
			break
		}
		bytesLeft = bytesLeft - 1
		disbursementIdBytes = append(disbursementIdBytes, b)
	}
	disbursementIdParts := strings.Split(string(disbursementIdBytes), ":")
	if len(disbursementIdParts) < 2 {
		return errors.New("invalid disbursement ID")
	}
	data.Claim.RewardID = disbursementIdParts[0]
	data.Claim.Specifier = strings.Join(disbursementIdParts[1:], ":")

	oracleBytes, err := decoder.ReadNBytes(20)
	if err != nil {
		return err
	}
	data.Claim.AntiAbuseOracleEthAddress = "0x" + hex.EncodeToString(oracleBytes)
	if data.Claim.AntiAbuseOracleEthAddress == "0x0000000000000000000000000000000000000000" {
		data.Claim.AntiAbuseOracleEthAddress = ""
	}

	// Skip unused bytes
	decoder.SkipBytes(uint(bytesLeft))

	// claim padding
	decoder.SkipBytes(uint(45))

	operatorBytes, err := decoder.ReadBytes(20)
	if err != nil {
		return err
	}
	data.OperatorEthAddress = "0x" + hex.EncodeToString(operatorBytes)

	return nil
}

type AttestationsAccountData struct {
	Version            uint8
	RewardManagerState solana.PublicKey
	Count              uint8
	Messages           []Attestation
}

func (data *AttestationsAccountData) UnmarshalWithDecoder(decoder *bin.Decoder) error {
	err := decoder.Decode(&data.Version)
	if err != nil {
		return err
	}
	err = decoder.Decode(&data.RewardManagerState)
	if err != nil {
		return err
	}
	err = decoder.Decode(&data.Count)
	if err != nil {
		return err
	}

	data.Messages = make([]Attestation, data.Count)
	for i := range data.Count {
		err = decoder.Decode(&data.Messages[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func deriveAuthorityAccount(programId solana.PublicKey, state solana.PublicKey) (solana.PublicKey, uint8, error) {
	seeds := make([][]byte, 1)
	seeds[0] = state.Bytes()[0:32]
	return solana.FindProgramAddress(seeds, programId)
}

func deriveSender(programId solana.PublicKey, authority solana.PublicKey, ethAddress common.Address) (solana.PublicKey, uint8, error) {
	senderSeedPrefix := []byte(SenderSeedPrefix)
	decodedEthAddress := ethAddress.Bytes()
	// Pad the eth address if necessary w/ leading 0
	senderSeed := make([]byte, len(senderSeedPrefix)+EthAddressByteLength)
	copy(senderSeed, senderSeedPrefix)
	copy(senderSeed[len(senderSeed)-len(decodedEthAddress):], decodedEthAddress)
	return solana.FindProgramAddress([][]byte{authority.Bytes()[0:32], senderSeed}, programId)
}

func deriveAttestationsAccount(programId solana.PublicKey, authority solana.PublicKey, disbursementId string) (solana.PublicKey, uint8, error) {
	attestationsSeed := make([]byte, len(AttestationsSeedPrefix)+len(disbursementId))
	copy(attestationsSeed, []byte(AttestationsSeedPrefix))
	copy(attestationsSeed[len([]byte(AttestationsSeedPrefix)):], disbursementId)
	return solana.FindProgramAddress([][]byte{authority.Bytes()[0:32], attestationsSeed}, programId)
}

func deriveDisbursement(programId solana.PublicKey, authority solana.PublicKey, disbursementId string) (solana.PublicKey, uint8, error) {
	disbursementSeed := make([]byte, len(DisbursementSeedPrefix)+len(disbursementId))
	copy(disbursementSeed, []byte(DisbursementSeedPrefix))
	copy(disbursementSeed[len([]byte(DisbursementSeedPrefix)):], disbursementId)
	return solana.FindProgramAddress([][]byte{authority.Bytes()[0:32], disbursementSeed}, programId)
}
