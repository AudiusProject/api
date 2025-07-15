package indexer

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/gagliardetto/solana-go"
	"go.uber.org/zap"
)

type parsedPurchaseMemo struct {
	ContentType           string
	ContentId             int
	ValidAfterBlocknumber int
	BuyerUserId           int
	AccessType            string
}

func (m parsedPurchaseMemo) String() string {
	return fmt.Sprintf("%s:%d:%d:%d:%s", m.ContentType, m.ContentId, m.ValidAfterBlocknumber, m.BuyerUserId, m.AccessType)
}

func ParsePurchaseMemo(memo []byte) (parsedPurchaseMemo, error) {
	parts := strings.Split(string(memo), ":")
	if len(parts) > 3 {
		contentType := parts[0]

		contentId, err := strconv.Atoi(parts[1])
		if err != nil {
			return parsedPurchaseMemo{}, fmt.Errorf("failed to parse contentId: %w", err)
		}

		validAfterBlocknumber, err := strconv.Atoi(parts[2])
		if err != nil {
			return parsedPurchaseMemo{}, fmt.Errorf("failed to parse validAfterBlocknumber: %w", err)
		}

		buyerUserId, err := strconv.Atoi(parts[3])
		if err != nil {
			return parsedPurchaseMemo{}, fmt.Errorf("failed to parse buyerUserId: %w", err)
		}

		accessType := "stream"
		if len(parts) > 4 {
			accessType = parts[4]
		}
		parsed := parsedPurchaseMemo{
			ContentType:           contentType,
			ContentId:             contentId,
			ValidAfterBlocknumber: validAfterBlocknumber,
			BuyerUserId:           buyerUserId,
			AccessType:            accessType,
		}
		return parsed, nil
	}
	return parsedPurchaseMemo{}, errors.New("not a purchase memo")
}

func findNextPurchaseMemo(tx *solana.Transaction, instructionIndex int, logger *zap.Logger) (parsedPurchaseMemo, bool) {
	for i := instructionIndex; i < len(tx.Message.Instructions); i++ {
		inst := tx.Message.Instructions[i]
		programId := tx.Message.AccountKeys[inst.ProgramIDIndex]
		if programId.Equals(solana.MemoProgramID) || programId.Equals(OLD_MEMO_PROGRAM_ID) {
			parsed, err := ParsePurchaseMemo(inst.Data)
			if err != nil {
				if logger != nil {
					logger.Warn("failed to parse purchase memo", zap.Error(err), zap.String("memo", string(inst.Data)))
				}
				continue
			}
			return parsed, true
		}
	}
	return parsedPurchaseMemo{}, false
}

type parsedLocationMemo struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
}

func ParseLocationMemo(memo []byte) (parsedLocationMemo, error) {
	if len(memo) > 3 && string(memo[0:3]) == "geo" {
		var parsed parsedLocationMemo
		err := json.Unmarshal(memo[4:], &parsed)
		if err != nil {
			return parsedLocationMemo{}, err
		}
		return parsed, nil
	}
	return parsedLocationMemo{}, errors.New("not a location memo")
}

func findNextLocationMemo(tx *solana.Transaction, instructionIndex int, logger *zap.Logger) parsedLocationMemo {
	for i := instructionIndex; i < len(tx.Message.Instructions); i++ {
		inst := tx.Message.Instructions[i]
		programId := tx.Message.AccountKeys[inst.ProgramIDIndex]
		if programId.Equals(solana.MemoProgramID) || programId.Equals(OLD_MEMO_PROGRAM_ID) {
			parsed, err := ParseLocationMemo(inst.Data)
			if err != nil {
				if logger != nil {
					logger.Warn("failed to parse location memo", zap.Error(err), zap.String("memo", string(inst.Data)))
				}
				continue
			}
			return parsed
		}
	}
	return parsedLocationMemo{}
}
