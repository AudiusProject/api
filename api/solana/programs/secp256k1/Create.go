package secp256k1

import (
	"fmt"

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

type Create struct {
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
	EthAddress       []byte
	Message          []byte
	Signature        []byte
	InstructionIndex uint8
}

func (data *Secp256k1SignatureData) String() string {
	return fmt.Sprintf("EthAddress: %x, Message: %x, Signature: %x, Instruction Index: %d", data.EthAddress, data.Message, data.Signature, data.InstructionIndex)
}

func NewCreateBuilder() *Create {
	nd := &Create{SignatureDatas: make([]Secp256k1SignatureData, 0)}
	return nd
}

func (inst *Create) AddSignatureData(ethAddress []byte, message []byte, signature []byte, instructionIndex uint8) *Create {
	inst.SignatureDatas = append(inst.SignatureDatas, Secp256k1SignatureData{
		EthAddress:       ethAddress,
		Message:          message,
		Signature:        signature,
		InstructionIndex: instructionIndex,
	})
	fmt.Println("AddSignatureData:", inst.SignatureDatas)
	return inst
}

func (obj *Create) SetAccounts(accounts []*solana.AccountMeta) error {
	return nil
}

func (slice Create) GetAccounts() (accounts []*solana.AccountMeta) {
	return
}

func (inst Create) Build() *Instruction {
	return &Instruction{BaseVariant: ag_binary.BaseVariant{
		Impl:   inst,
		TypeID: ag_binary.NoTypeIDDefaultID,
	}}
}

func (obj Create) MarshalWithEncoder(encoder *ag_binary.Encoder) (err error) {
	numSignatures := len(obj.SignatureDatas)
	err = encoder.Encode(uint8(numSignatures))
	if err != nil {
		return err
	}

	for _, signatureData := range obj.SignatureDatas {
		ethAddressOffset := encoder.Written()
		signatureOffset := ethAddressOffset + len(signatureData.EthAddress)
		messageDataOffset := signatureOffset + len(signatureData.Signature) + 1

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

		err = encoder.WriteBytes(signatureData.EthAddress, false)
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

func (obj *Create) UnmarshalWithDecoder(decoder *ag_binary.Decoder) (err error) {
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
		signature, err := decoder.ReadBytes(int(offsets.MessageDataOffset) - int(offsets.SignatureOffset) - 1)
		if err != nil {
			return err
		}
		message, err := decoder.ReadBytes(int(offsets.MessageDataSize))
		if err != nil {
			return err
		}

		fmt.Println(offsets)

		obj.AddSignatureData(ethAddress, message, signature, offsets.EthAddressInstructionIndex)
	}
	return nil
}

func NewCreateInstruction(ethAddress []byte, message []byte, signature []byte, instructionIndex uint8) *Create {
	return NewCreateBuilder().AddSignatureData(ethAddress, message, signature, instructionIndex)
}
