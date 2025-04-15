// copy-pasted from mediorum placement.go

package rendezvous

import (
	"bytes"
	"crypto/sha256"
	"io"
	"math/rand"
	"net/url"
	"os"
	"slices"
	"sort"
	"strings"
)

var GlobalHasher *RendezvousHasher

func init() {
	hostList := []string{
		"https://creatornode.audius.co",
		"https://creatornode2.audius.co",
		"https://creatornode3.audius.co",
		"https://audius-content-1.figment.io",
		"https://creatornode.audius.prod-eks-ap-northeast-1.staked.cloud",
		"https://audius-content-2.figment.io",
		"https://audius-content-3.figment.io",
		"https://audius-content-4.figment.io",
		"https://audius-content-5.figment.io",
		"https://creatornode.audius1.prod-eks-ap-northeast-1.staked.cloud",
		"https://creatornode.audius2.prod-eks-ap-northeast-1.staked.cloud",
		"https://creatornode.audius3.prod-eks-ap-northeast-1.staked.cloud",
		"https://audius-content-6.figment.io",
		"https://audius-content-7.figment.io",
		"https://audius-content-8.figment.io",
		"https://audius-content-9.figment.io",
		"https://audius-content-10.figment.io",
		"https://audius-content-11.figment.io",
		"https://content.grassfed.network",
		"https://blockdaemon-audius-content-01.bdnodes.net",
		"https://audius-content-1.cultur3stake.com",
		"https://audius-content-2.cultur3stake.com",
		"https://audius-content-3.cultur3stake.com",
		"https://audius-content-4.cultur3stake.com",
		"https://audius-content-5.cultur3stake.com",
		"https://audius-content-6.cultur3stake.com",
		"https://audius-content-7.cultur3stake.com",
		"https://blockdaemon-audius-content-02.bdnodes.net",
		"https://blockdaemon-audius-content-03.bdnodes.net",
		"https://blockdaemon-audius-content-04.bdnodes.net",
		"https://blockdaemon-audius-content-05.bdnodes.net",
		"https://blockdaemon-audius-content-06.bdnodes.net",
		"https://blockdaemon-audius-content-07.bdnodes.net",
		"https://blockdaemon-audius-content-08.bdnodes.net",
		"https://blockdaemon-audius-content-09.bdnodes.net",
		"https://audius-content-8.cultur3stake.com",
		"https://blockchange-audius-content-01.bdnodes.net",
		"https://blockchange-audius-content-02.bdnodes.net",
		"https://blockchange-audius-content-03.bdnodes.net",
		"https://audius-content-9.cultur3stake.com",
		"https://audius-content-10.cultur3stake.com",
		"https://audius-content-11.cultur3stake.com",
		"https://audius-content-12.cultur3stake.com",
		"https://audius-content-13.cultur3stake.com",
		"https://audius-content-14.cultur3stake.com",
		"https://audius-content-15.cultur3stake.com",
		"https://audius-content-16.cultur3stake.com",
		"https://audius-content-17.cultur3stake.com",
		"https://audius-content-18.cultur3stake.com",
		"https://audius-content-12.figment.io",
		"https://cn0.mainnet.audiusindex.org",
		"https://cn1.mainnet.audiusindex.org",
		"https://cn2.mainnet.audiusindex.org",
		"https://cn3.mainnet.audiusindex.org",
		"https://audius-content-13.figment.io",
		"https://audius-content-14.figment.io",
		"https://cn4.mainnet.audiusindex.org",
		"https://audius-creator-1.theblueprint.xyz",
		"https://audius-creator-2.theblueprint.xyz",
		"https://audius-creator-3.theblueprint.xyz",
		"https://audius-creator-4.theblueprint.xyz",
		"https://audius-creator-5.theblueprint.xyz",
		"https://audius-creator-6.theblueprint.xyz",
		"https://creatornode.audius8.prod-eks-ap-northeast-1.staked.cloud",
		"https://cn1.stuffisup.com",
		"https://audius-cn1.tikilabs.com",
		"https://audius-creator-7.theblueprint.xyz",
		"https://cn1.shakespearetech.com",
		"https://cn2.shakespearetech.com",
		"https://cn3.shakespearetech.com",
		"https://audius-creator-8.theblueprint.xyz",
		"https://audius-creator-9.theblueprint.xyz",
		"https://audius-creator-10.theblueprint.xyz",
		"https://audius-creator-11.theblueprint.xyz",
		"https://audius-creator-12.theblueprint.xyz",
		"https://audius-creator-13.theblueprint.xyz",
	}
	if os.Getenv("ENV") == "stage" {
		hostList = []string{
			"https://creatornode10.staging.audius.co",
			"https://creatornode11.staging.audius.co",
			"https://creatornode12.staging.audius.co",
			"https://creatornode5.staging.audius.co",
			"https://creatornode6.staging.audius.co",
			"https://creatornode7.staging.audius.co",
			"https://creatornode8.staging.audius.co",
			"https://creatornode9.staging.audius.co",
		}
	}

	GlobalHasher = NewRendezvousHasher(hostList)
}

type HostTuple struct {
	host  string
	score []byte
}

type HostTuples []HostTuple

func (s HostTuples) Len() int      { return len(s) }
func (s HostTuples) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s HostTuples) Less(i, j int) bool {
	c := bytes.Compare(s[i].score, s[j].score)
	if c == 0 {
		return s[i].host < s[j].host
	}
	return c == -1
}

func NewRendezvousHasher(hosts []string) *RendezvousHasher {
	deadHosts := "https://content.grassfed.network/"
	liveHosts := make([]string, 0, len(hosts))
	for _, h := range hosts {
		// dead host
		if strings.Contains(deadHosts, h) {
			continue
		}

		// invalid url
		if _, err := url.Parse(h); err != nil {
			continue
		}

		// duplicate entry
		if slices.Contains(liveHosts, h) {
			continue
		}

		liveHosts = append(liveHosts, h)
	}
	return &RendezvousHasher{
		hosts: liveHosts,
	}
}

type RendezvousHasher struct {
	hosts []string
}

func (rh *RendezvousHasher) Rank(key string) []string {
	tuples := make(HostTuples, len(rh.hosts))
	keyBytes := []byte(key)
	hasher := sha256.New()
	for idx, host := range rh.hosts {
		hasher.Reset()
		io.WriteString(hasher, host)
		hasher.Write(keyBytes)
		tuples[idx] = HostTuple{host, hasher.Sum(nil)}
	}
	sort.Sort(tuples)
	result := make([]string, len(rh.hosts))
	for idx, tup := range tuples {
		result[idx] = tup.host
	}
	return result
}

// Get a replica set of 3 nodes with random order
func (rh *RendezvousHasher) ReplicaSet3(key string) (string, []string) {
	ranked := rh.Rank(key)
	n := min(len(ranked), 3)
	if n == 0 {
		return "", []string{}
	}

	candidates := append([]string(nil), ranked[:n]...)
	rand.Shuffle(n, func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	return candidates[0], candidates[1:]
}
