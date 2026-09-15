package main

import (
	"context"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
)

// getProgramAccountsV2 is a thin wrapper around the getProgramAccountsV2 RPC
// method (a paginated getProgramAccounts). Upstream solana-go does not expose
// it, so it lives here rather than in a fork of the SDK.

type getProgramAccountsV2Opts struct {
	rpc.GetProgramAccountsOpts

	Limit            *uint64 `json:"limit,omitempty"`
	PaginationKey    *string `json:"paginationKey,omitempty"`
	ChangedSinceSlot *uint64 `json:"changedSinceSlot,omitempty"`
}

type getProgramAccountsV2Result struct {
	Accounts      []*rpc.KeyedAccount `json:"accounts"`
	PaginationKey *string             `json:"paginationKey,omitempty"`
	TotalResults  *uint64             `json:"totalResults,omitempty"`
}

func getProgramAccountsV2WithOpts(
	ctx context.Context,
	client *rpc.Client,
	publicKey solana.PublicKey,
	opts *getProgramAccountsV2Opts,
) (out getProgramAccountsV2Result, err error) {
	obj := map[string]any{
		"encoding": "base64",
	}
	if opts != nil {
		if opts.Commitment != "" {
			obj["commitment"] = string(opts.Commitment)
		}
		if len(opts.Filters) != 0 {
			obj["filters"] = opts.Filters
		}
		if opts.Encoding != "" {
			obj["encoding"] = opts.Encoding
		}
		if opts.DataSlice != nil {
			obj["dataSlice"] = map[string]any{
				"offset": opts.DataSlice.Offset,
				"length": opts.DataSlice.Length,
			}
		}
		if opts.Limit != nil {
			obj["limit"] = opts.Limit
		}
		if opts.PaginationKey != nil {
			obj["paginationKey"] = opts.PaginationKey
		}
		if opts.ChangedSinceSlot != nil {
			obj["changedSinceSlot"] = opts.ChangedSinceSlot
		}
	}

	params := []any{publicKey, obj}
	err = client.RPCCallForInto(ctx, &out, "getProgramAccountsV2", params)
	return
}
