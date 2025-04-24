package config

import (
	"log"
	"os"

	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	Env                           string
	DbUrl                         string
	Nodes                         []string
	DeadNodes                     []string
	DelegatePrivateKey            string
	AxiomToken                    string
	AxiomDataset                  string
	PythonUpstreams               []string
	NetworkTakeRate               float64
	StakingBridgeUsdcPayoutWallet string
	SolanaConfig                  SolanaConfig
	AntiAbuseOracles              []string
}

var Cfg = Config{
	Env:                           os.Getenv("ENV"),
	DbUrl:                         os.Getenv("discoveryDbUrl"),
	DelegatePrivateKey:            os.Getenv("delegatePrivateKey"),
	AxiomToken:                    os.Getenv("axiomToken"),
	AxiomDataset:                  os.Getenv("axiomDataset"),
	NetworkTakeRate:               10,
	StakingBridgeUsdcPayoutWallet: "7vGA3fcjvxa3A11MAxmyhFtYowPLLCNyvoxxgN3NN2Vf",
	SolanaConfig:                  SolCfg,
}

func init() {
	switch env := os.Getenv("ENV"); env {
	case "dev":
		fallthrough
	case "development":
		fallthrough
	case "":
		Cfg.AntiAbuseOracles = []string{"http://audius-protocol-discovery-provider-1"}
	case "stage":
		fallthrough
	case "staging":
		if Cfg.DelegatePrivateKey == "" {
			log.Fatalf("Missing required %s env var: delegatePrivateKey", env)
		}
		Cfg.AntiAbuseOracles = []string{"https://discoveryprovider.staging.audius.co"}
		Cfg.PythonUpstreams = []string{
			"https://discoveryprovider.staging.audius.co",
			"https://discoveryprovider2.staging.audius.co",
			"https://discoveryprovider3.staging.audius.co",
			"https://discoveryprovider5.staging.audius.co",
		}
		Cfg.Nodes = []string{
			"https://creatornode11.staging.audius.co",
			"https://creatornode12.staging.audius.co",
			"https://creatornode5.staging.audius.co",
			"https://creatornode6.staging.audius.co",
			"https://creatornode7.staging.audius.co",
			"https://creatornode9.staging.audius.co",
		}
		Cfg.DeadNodes = []string{}
	case "prod":
		fallthrough
	case "production":
		if Cfg.DelegatePrivateKey == "" {
			log.Fatalf("Missing required %s env var: delegatePrivateKey", env)
		}
		Cfg.AntiAbuseOracles = []string{"https://discoveryprovider.audius.co"}
		Cfg.PythonUpstreams = []string{
			"https://discoveryprovider.audius.co",
			"https://discoveryprovider2.audius.co",
			"https://discoveryprovider3.audius.co",
		}
		Cfg.Nodes = []string{
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
		Cfg.DeadNodes = []string{
			"https://content.grassfed.network",
		}
	default:
		log.Fatalf("Unknown environment: %s", env)
	}
}
