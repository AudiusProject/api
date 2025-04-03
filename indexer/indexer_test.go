package indexer

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"testing"

	"github.com/AudiusProject/audiusd/pkg/core/gen/core_proto"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func checkErr(err error) {
	if err != nil {
		panic(err)
	}
}

var (
	ci *CoreIndexer
)

func TestMain(m *testing.M) {
	ctx := context.Background()
	var err error

	// create a test db from template
	{
		conn, err := pgx.Connect(ctx, "postgres://postgres:example@localhost:21300/postgres")
		checkErr(err)

		_, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS indexer_test")
		checkErr(err)

		_, err = conn.Exec(ctx, "CREATE DATABASE indexer_test TEMPLATE postgres")
		checkErr(err)
	}

	ci, err = NewIndexer(CoreIndexerConfig{
		DbUrl: "postgres://postgres:example@localhost:21300/indexer_test",
	})
	checkErr(err)

	// relax schema a bit...
	_, err = ci.pool.Exec(ci.ctx, `
	alter table follows drop constraint follows_blocknumber_fkey;
	alter table reposts drop constraint reposts_blocknumber_fkey;
	`)
	checkErr(err)

	code := m.Run()

	// shutdown()
	os.Exit(code)
}

func TestIndexer(t *testing.T) {
	assert.Equal(t, 1, 1)
}

func TestReadProtoFile(t *testing.T) {

	file, err := os.Open("testdata/take1.pb")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	for {
		var length uint32
		err := binary.Read(file, binary.LittleEndian, &length)
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			panic(err)
		}

		data := make([]byte, length)
		_, err = file.Read(data)
		if err != nil {
			fmt.Println(err)
			break
		}

		var signedTx core_proto.SignedTransaction
		if err := proto.Unmarshal(data, &signedTx); err != nil {
			assert.NoError(t, err)
		}

		err = ci.handleTx(&signedTx)
		assert.NoError(t, err)

	}

}
