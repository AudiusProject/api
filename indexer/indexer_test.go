package indexer

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"bridgerton.audius.co/database"
	core_proto "github.com/AudiusProject/audiusd/pkg/api/core/v1"
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
	var err error
	pool := database.CreateTestDatabase(nil, "test_indexer")
	ci, err = NewIndexer(CoreIndexerConfig{
		DbUrl: pool.Config().ConnString(),
	})
	checkErr(err)
	ci.pool.Close()
	ci.pool = pool

	// relax schema a bit...
	_, err = ci.pool.Exec(ci.ctx, `
	alter table follows drop constraint follows_blocknumber_fkey;
	alter table reposts drop constraint reposts_blocknumber_fkey;
	alter table saves drop constraint saves_blocknumber_fkey;
	alter table notification_seen drop constraint notification_seen_blocknumber_fkey;
	alter table track_downloads drop constraint track_downloads_blocknumber_fkey;
	alter table comments drop constraint comments_blocknumber_fkey;
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
	t.Skip()

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

func assertCount(t *testing.T, expected int, sql string) {
	count := -1
	err := ci.pool.QueryRow(ci.ctx, sql).Scan(&count)
	assert.NoError(t, err)
	assert.Equal(t, expected, count)
}

func toMetadata(data map[string]any) string {
	b, err := json.Marshal(GenericMetadata{Data: data})
	if err != nil {
		panic(err)
	}
	return string(b)
}
