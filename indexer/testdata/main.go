package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"

	core_proto "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/proto"

	_ "github.com/joho/godotenv/autoload"
)

func writeStuff() {
	ctx := context.Background()
	dbUrl := os.Getenv("discoveryDbUrl")
	pool, err := pgxpool.New(context.Background(), dbUrl)
	if err != nil {
		panic(err)
	}

	file, err := os.Create("indexer/testdata/take1.pb")
	if err != nil {
		panic(err)
	}

	startBlock := 0
	count := 0

	for startBlock < 5000 {
		fmt.Println("START BLOCK", startBlock)

		rows, err := pool.Query(ctx, `
		select block_id, transaction
		from core_transactions
		where block_id > $1
		order by block_id asc
		limit 1000`, startBlock)
		if err != nil {
			panic(err)
		}

		var blockId int
		var pb []byte
		_, err = pgx.ForEachRow(rows, []any{&blockId, &pb}, func() error {
			startBlock = blockId

			// Serialize the protobuf message length and data to the file
			length := uint32(len(pb))
			if err := binary.Write(file, binary.LittleEndian, length); err != nil {
				return err
			}
			if _, err := file.Write(pb); err != nil {
				return err
			}
			count++

			var signedTx core_proto.SignedTransaction
			if err := proto.Unmarshal(pb, &signedTx); err != nil {
				return err
			}

			switch signedTx.GetTransaction().(type) {
			case *core_proto.SignedTransaction_Plays:
				// play := signedTx.GetPlays()
				// fmt.Println("PLAY", play)
			case *core_proto.SignedTransaction_ManageEntity:
				// em := signedTx.GetManageEntity()
				// fmt.Println("-------------")
				// fmt.Println("EM", em.UserId, em.Action, em.EntityType, em.EntityId)
				// fmt.Println(em.Metadata)
			}

			return nil
		})
		if err != nil {
			panic(err)
		}

	}

	file.Close()

	fmt.Println("WROTE", count)
}

func main() {
	writeStuff()
	// readStuff()
}
