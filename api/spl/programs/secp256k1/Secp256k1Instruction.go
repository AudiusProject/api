package secp256k1

import (
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	ag_binary "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

// https://docs.rs/solana-secp256k1-program/latest/src/solana_secp256k1_program/lib.rs.html#783-786
const (
	HashedPubkeySerializedSize     = 20
	SignaturesSerializedSize       = 64
	SignatureOffsetsSerializedSize = 11
	DataStart                      = SignatureOffsetsSerializedSize + 1
)

type Secp256k1Instruction struct {
	SignatureDatas []Secp256k1SignatureData
}

// https://docs.rs/solana-secp256k1-program/latest/src/solana_secp256k1_program/lib.rs.html#788-810
type Secp256k1SignatureOffsets struct {
	SignatureOffset            uint16
	SignatureInstructionIndex  uint8
	EthAddressOffset           uint16
	EthAddressInstructionIndex uint8
	MessageDataOffset          uint16
	MessageDataSize            uint16
	MessageInstructionIndex    uint8
}

type Secp256k1SignatureData struct {
	EthAddress       common.Address
	Message          []byte
	Signature        []byte
	InstructionIndex uint8
}

func NewSecp256k1InstructionBuilder() *Secp256k1Instruction {
	nd := &Secp256k1Instruction{SignatureDatas: make([]Secp256k1SignatureData, 0)}
	return nd
}

func (inst *Secp256k1Instruction) AddSignatureData(ethAddress common.Address, message []byte, signature []byte, instructionIndex uint8) *Secp256k1Instruction {
	inst.SignatureDatas = append(inst.SignatureDatas, Secp256k1SignatureData{
		EthAddress:       ethAddress,
		Message:          message,
		Signature:        signature,
		InstructionIndex: instructionIndex,
	})
	return inst
}

func (obj *Secp256k1Instruction) SetAccounts(accounts []*solana.AccountMeta) error {
	return nil
}

func (slice Secp256k1Instruction) GetAccounts() (accounts []*solana.AccountMeta) {
	return
}

func (inst Secp256k1Instruction) Validate() error {
	for _, sigData := range inst.SignatureDatas {
		hash := crypto.Keccak256(sigData.Message)
		recoveredPubkey, err := crypto.SigToPub(hash, sigData.Signature)
		if err != nil {
			return err
		}
		recoveredEthAddress := crypto.PubkeyToAddress(*recoveredPubkey)
		same := recoveredEthAddress.Cmp(sigData.EthAddress) == 0
		if !same {
			return errors.New("signature invalid")
		}
	}
	return nil
}

func (inst Secp256k1Instruction) Build() *Instruction {
	return &Instruction{BaseVariant: ag_binary.BaseVariant{
		Impl:   inst,
		TypeID: ag_binary.NoTypeIDDefaultID,
	}}
}

func (obj Secp256k1Instruction) MarshalWithEncoder(encoder *ag_binary.Encoder) (err error) {
	numSignatures := len(obj.SignatureDatas)
	err = encoder.Encode(uint8(numSignatures))
	if err != nil {
		return err
	}

	for _, signatureData := range obj.SignatureDatas {
		ethAddressOffset := encoder.Written() + SignatureOffsetsSerializedSize
		signatureOffset := ethAddressOffset + len(signatureData.EthAddress)
		messageDataOffset := signatureOffset + len(signatureData.Signature)

		offsets := Secp256k1SignatureOffsets{
			SignatureOffset:            uint16(signatureOffset),
			SignatureInstructionIndex:  uint8(signatureData.InstructionIndex),
			EthAddressOffset:           uint16(ethAddressOffset),
			EthAddressInstructionIndex: uint8(signatureData.InstructionIndex),
			MessageDataOffset:          uint16(messageDataOffset),
			MessageDataSize:            uint16(len(signatureData.Message)),
			MessageInstructionIndex:    uint8(signatureData.InstructionIndex),
		}
		err := encoder.Encode(offsets)
		if err != nil {
			return err
		}

		err = encoder.WriteBytes(signatureData.EthAddress.Bytes(), false)
		if err != nil {
			return err
		}

		err = encoder.WriteBytes(signatureData.Signature, false)
		if err != nil {
			return err
		}

		err = encoder.WriteBytes(signatureData.Message, false)
		if err != nil {
			return err
		}

	}

	return nil
}

func (obj *Secp256k1Instruction) UnmarshalWithDecoder(decoder *ag_binary.Decoder) (err error) {
	numSignatures, err := decoder.ReadUint8()
	if err != nil {
		return err
	}

	for range numSignatures {
		offsets := &Secp256k1SignatureOffsets{}
		err = decoder.Decode(offsets)
		if err != nil {
			return err
		}

		ethAddress, err := decoder.ReadBytes(int(offsets.SignatureOffset) - int(offsets.EthAddressOffset))
		if err != nil {
			return err
		}
		signature, err := decoder.ReadBytes(int(offsets.MessageDataOffset) - int(offsets.SignatureOffset))
		if err != nil {
			return err
		}
		message, err := decoder.ReadBytes(int(offsets.MessageDataSize))
		if err != nil {
			return err
		}

		obj.AddSignatureData(common.BytesToAddress(ethAddress), message, signature, offsets.EthAddressInstructionIndex)
	}
	return nil
}

func NewSecp256k1Instruction(ethAddress common.Address, message []byte, signature []byte, instructionIndex uint8) *Secp256k1Instruction {
	return NewSecp256k1InstructionBuilder().AddSignatureData(ethAddress, message, signature, instructionIndex)
}
