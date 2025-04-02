package main

import (
	"context"
	"fmt"
	"os"

	"github.com/AudiusProject/audiusd/pkg/core/gen/core_proto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/joho/godotenv/autoload"
	"google.golang.org/protobuf/proto"
)

func main() {
	ctx := context.Background()
	dbUrl := os.Getenv("discoveryDbUrl")
	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		panic(err)
	}

	startBlock := 0

	for {
		fmt.Println("START BLOCK", startBlock)

		rows, err := pool.Query(ctx, `
		select block_id, transaction
		from core_transactions
		where block_id > $1
		order by block_id asc
		limit 100`, startBlock)
		if err != nil {
			panic(err)
		}

		var blockId int
		var pb []byte
		_, err = pgx.ForEachRow(rows, []any{&blockId, &pb}, func() error {
			startBlock = blockId

			var signedTx core_proto.SignedTransaction
			if err := proto.Unmarshal(pb, &signedTx); err != nil {
				return err
			}

			switch signedTx.GetTransaction().(type) {
			case *core_proto.SignedTransaction_Plays:
				play := signedTx.GetPlays()
				fmt.Println("PLAY", play)
			case *core_proto.SignedTransaction_ManageEntity:
				em := signedTx.GetManageEntity()
				fmt.Println("EM", em)
			}

			return nil
		})
		if err != nil {
			panic(err)
		}

	}
}
