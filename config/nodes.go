package config

type Node struct {
	// Full origin URL (including protoocol)
	Endpoint string
	// The individual wallet address for the node
	DelegateOwnerWallet string
	// The wallet address for the node owner/operator
	OwnerWallet string
	// Discovery Nodes have storage disabled
	IsStorageDisabled bool
}

var (
	ProdNodes = []Node{
		{
			DelegateOwnerWallet: "0x7db3789e5E2154569e802945ECF2cC92e0994841",
			Endpoint:            "https://audius-metadata-1.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x4E2C78d0d3303ed459BF8a3CD87f11A6bc936140",
			Endpoint:            "https://audius-metadata-2.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xE77C7679ED77b175F935755EEb3a421635AF07EC",
			Endpoint:            "https://audius-discovery-1.altego.net",
			OwnerWallet:         "0xA9cB9d043d4841dE83C70556FF0Bd4949C15b5Eb",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xE515A7B710e7CBB55F0fB73fc56c15Ad9b36Af9B",
			Endpoint:            "https://dn-jpn.audius.metadata.fyi",
			OwnerWallet:         "0x067D4f5229b453C3743023135Ecc76f8d27b9008",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xF2897993951d53a7E3eb2242D6A14D2028140DC8",
			Endpoint:            "https://discoveryprovider3.audius.co",
			OwnerWallet:         "0xe5b256d302ea2f4e04B8F3bfD8695aDe147aB68d",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xc97d40C0B992882646D64814151941A1c520b460",
			Endpoint:            "https://discoveryprovider2.audius.co",
			OwnerWallet:         "0xe5b256d302ea2f4e04B8F3bfD8695aDe147aB68d",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xf1a1Bd34b2Bc73629aa69E50E3249E89A3c16786",
			Endpoint:            "https://discoveryprovider.audius.co",
			OwnerWallet:         "0xe5b256d302ea2f4e04B8F3bfD8695aDe147aB68d",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xE019F1Ad9803cfC83e11D37Da442c9Dc8D8d82a6",
			Endpoint:            "https://audius-metadata-3.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xf7441A14A31199744Bf8e7b79405c5446C120D0f",
			Endpoint:            "https://audius-metadata-4.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x2CD66a3931C36596efB037b06753476dcE6B4e86",
			Endpoint:            "https://dn1.monophonic.digital",
			OwnerWallet:         "0x6470Daf3bd32f5014512bCdF0D02232f5640a5BD",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x4a3D65647A8Ac41Ef7bdF13D1F171aA97a15ae4b",
			Endpoint:            "https://dn-usa.audius.metadata.fyi",
			OwnerWallet:         "0x067D4f5229b453C3743023135Ecc76f8d27b9008",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xaC69a173aC26E2daB8663E210eD87a222Ec3945B",
			Endpoint:            "https://discovery-us-01.audius.openplayer.org",
			OwnerWallet:         "0x55fc79f85eEc693A65f79DB463dc3E6831364Bce",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x422541273087beC833c57D3c15B9e17F919bFB1F",
			Endpoint:            "https://dn2.monophonic.digital",
			OwnerWallet:         "0x6470Daf3bd32f5014512bCdF0D02232f5640a5BD",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xA9cB9d043d4841dE83C70556FF0Bd4949C15b5Eb",
			Endpoint:            "https://audius-discovery-2.altego.net",
			OwnerWallet:         "0xA9cB9d043d4841dE83C70556FF0Bd4949C15b5Eb",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x42D35a2f33ba468fA9eB6FFEA4b975F182957556",
			Endpoint:            "https://dn1.nodeoperator.io",
			OwnerWallet:         "0x858e345E9DC681357ecd44bA285e04180c481fF6",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xA9cB9d043d4841dE83C70556FF0Bd4949C15b5Eb",
			Endpoint:            "https://audius-discovery-3.altego.net",
			OwnerWallet:         "0xA9cB9d043d4841dE83C70556FF0Bd4949C15b5Eb",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x67154199E79bEcd2A1f34f89d6AF962CF9863945",
			Endpoint:            "https://dn1.matterlightblooming.xyz",
			OwnerWallet:         "0xb5F5280e275eCa21f167d870d054b90C9C7e6669",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xb4c7895739062A54F33998D65eF90afb3689d765",
			Endpoint:            "https://discovery.grassfed.network",
			OwnerWallet:         "0x57B1d346CDe1d2fA740F310b0d358d07d7c49547",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xD3Fe61E45956a3BCE819DD6fC8091E8dBb054cFD",
			Endpoint:            "https://audius-discovery-3.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x1b05E1a7E221785BE8D9E7f397962Df9c5539464",
			Endpoint:            "https://audius-discovery-4.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x120cd44EE33E17C2F7A6b95dAA0920342f534E21",
			Endpoint:            "https://audius-discovery-5.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x6B696B2ae65A885660c3a1DA44b6306509CC2350",
			Endpoint:            "https://audius-discovery-7.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x2eDfC1ecD381c991DfcAa6951d7766F4Dbba8CA2",
			Endpoint:            "https://audius-discovery-8.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xd8091A289BEf13b5407082Bb66000ccA47e7e34C",
			Endpoint:            "https://audius-discovery-9.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x4086DBFb51E451fD1AEeC778FFb884201c944E94",
			Endpoint:            "https://audius-discovery-10.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x6CAA3671162bC259094Ea4451d0d16792431C37a",
			Endpoint:            "https://discovery-au-02.audius.openplayer.org",
			OwnerWallet:         "0x55fc79f85eEc693A65f79DB463dc3E6831364Bce",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xD3A697f1084e50c19b19a8859E3d746893152c29",
			Endpoint:            "https://disc-lon01.audius.hashbeam.com",
			OwnerWallet:         "0x1BD9D60a0103FF2fA25169918392f118Bc616Dc9",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x70256629E87b41105F997878D2Db749a78a5B695",
			Endpoint:            "https://blockdaemon-audius-discovery-01.bdnodes.net",
			OwnerWallet:         "0x091D2190e93A9C09f99dA05ec3F82ef5D8aa4a07",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x060e48dd69960829Fb23CB41eB2DFDAc57948FAd",
			Endpoint:            "https://blockdaemon-audius-discovery-02.bdnodes.net",
			OwnerWallet:         "0x091D2190e93A9C09f99dA05ec3F82ef5D8aa4a07",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x2416D78b3cc41467c22578dEE7CA90450EB6526e",
			Endpoint:            "https://blockdaemon-audius-discovery-03.bdnodes.net",
			OwnerWallet:         "0xEe39B44cE36384157585C19df17d9B28D5637C4D",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xbD0548Ce77e69CE22Af591A4155162A08fDDEC3d",
			Endpoint:            "https://blockdaemon-audius-discovery-04.bdnodes.net",
			OwnerWallet:         "0xEe39B44cE36384157585C19df17d9B28D5637C4D",
		},
		{
			DelegateOwnerWallet: "0xF5EA27b029D5579D344CFa558DDc3B76A39c98d3",
			Endpoint:            "https://blockdaemon-audius-discovery-05.bdnodes.net",
			OwnerWallet:         "0x447E3572B5511cc6ea0700e34D2443017D081d7e",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x4ACD4eb0F0992cBFf18d5Cb551f3d8790Db01c51",
			Endpoint:            "https://blockdaemon-audius-discovery-06.bdnodes.net",
			OwnerWallet:         "0x447E3572B5511cc6ea0700e34D2443017D081d7e",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xD207D8Eb95aA5b2595cF3EEA14308EB61A36ad21",
			Endpoint:            "https://blockchange-audius-discovery-01.bdnodes.net",
			OwnerWallet:         "0x59938DF0F43DC520404e4aafDdae688a455Be870",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xAB30eF276ADC2bE22CE58d75B4F4009173A73676",
			Endpoint:            "https://blockchange-audius-discovery-02.bdnodes.net",
			OwnerWallet:         "0x59938DF0F43DC520404e4aafDdae688a455Be870",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x048cFedf907c4C9dDD11ff882380906E78E84BbE",
			Endpoint:            "https://blockchange-audius-discovery-03.bdnodes.net",
			OwnerWallet:         "0x59938DF0F43DC520404e4aafDdae688a455Be870",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xC6f37525A2EBab1eb02B4c6ba302F402e4c5ad1C",
			Endpoint:            "https://audius-discovery-11.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x1354aFF85DfCeF324E8e40d356f53Cd5F0ED4b83",
			Endpoint:            "https://audius-discovery-12.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x6f43df165E57598Bd74A2D6ADD18ba4249ECd16B",
			Endpoint:            "https://audius-discovery-13.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x0d64915a5F498131474C9A569F0AE0164efB95B5",
			Endpoint:            "https://audius-discovery-14.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xD083A0fA8c2d84759f5383EE4655aAb9908E832c",
			Endpoint:            "https://audius-discovery-16.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x6C3d9f517a1768dDcDC5e37945e75CAD7A3dF6CC",
			Endpoint:            "https://audius-discovery-18.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x06D39081B2811fA7CbADC3D7c4e96889829cdec5",
			Endpoint:            "https://audius-discovery-17.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xE34CB31dadA68F046864054E7A500a370F67b973",
			Endpoint:            "https://audius-discovery-15.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xf83cA74d5E6AD3F2946754Fa0D1e5cE7670DB764",
			Endpoint:            "https://audius-discovery-6.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x7c125128B0917bDE12e6A0eDde8F7675d4ADF408",
			Endpoint:            "https://audius-discovery-2.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x8464c88502925a0076c381962F8B70b6EC892861",
			Endpoint:            "https://blockdaemon-audius-discovery-08.bdnodes.net",
			OwnerWallet:         "0x091D2190e93A9C09f99dA05ec3F82ef5D8aa4a07",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x69cfDc1AB75384f077E4E48cf0d6483C8fB9B8A2",
			Endpoint:            "https://audius-metadata-5.figment.io",
			OwnerWallet:         "0x700a11aE95E34fBC769f8EAD063403987Bd0C502",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xAA29e93f4008D977078957D8f041AEAeF7e1eeBc",
			Endpoint:            "https://dn1.stuffisup.com",
			OwnerWallet:         "0x3E2Cd6d498b412Da182Ef25837F72355f8918BE9",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xEDe07aCa59815fbaa75c4f813dCDD1390D371071",
			Endpoint:            "https://audius-discovery-1.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xCF3f359BfdE7bcAfE4bc058B6DFae51aBe204aB4",
			Endpoint:            "https://audius-discovery-2.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x8449169096550905B903b6803FB3b64285112603",
			Endpoint:            "https://audius-discovery-3.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x16e8DF288BF5DcD507615A715A2a6155F149a865",
			Endpoint:            "https://audius-discovery-4.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xE83699015c8eb793A0678eA7dC398ac58f7928c4",
			Endpoint:            "https://audius-nodes.com",
			OwnerWallet:         "0xE83699015c8eb793A0678eA7dC398ac58f7928c4",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x1cF73c5023572F2d5dc6BD3c5E4F24b4F3b6B76F",
			Endpoint:            "https://audius-dn1.tikilabs.com",
			OwnerWallet:         "0xe4882D9A38A2A1fc652996719AF0fb15CB968d0a",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xC7562a5CF872450744C3DC5cDb00e9f105D2EfDc",
			Endpoint:            "https://blockchange-audius-discovery-04.bdnodes.net",
			OwnerWallet:         "0x59938DF0F43DC520404e4aafDdae688a455Be870",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x319211E15876156BD992dd047587d0cd7b88Be77",
			Endpoint:            "https://blockchange-audius-discovery-05.bdnodes.net",
			OwnerWallet:         "0x59938DF0F43DC520404e4aafDdae688a455Be870",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xFD005a90cc8AF8B766F9F9cD95ee91921cC9286d",
			Endpoint:            "https://audius-disc1.nodemagic.com",
			OwnerWallet:         "0xf13612C7d6E31636eCC2b670d6F8a3CC50f68A48",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x5cA0d3a6590074B9fF31972824178f69e8dAB547",
			Endpoint:            "https://audius-disc2.nodemagic.com",
			OwnerWallet:         "0xf13612C7d6E31636eCC2b670d6F8a3CC50f68A48",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xc8d0C29B6d540295e8fc8ac72456F2f4D41088c8",
			Endpoint:            "https://creatornode.audius.co",
			OwnerWallet:         "0xe5b256d302ea2f4e04b8f3bfd8695ade147ab68d",
		},
		{
			DelegateOwnerWallet: "0xf686647E3737d595C60c6DE2f5F90463542FE439",
			Endpoint:            "https://creatornode2.audius.co",
			OwnerWallet:         "0xe5b256d302ea2f4e04b8f3bfd8695ade147ab68d",
		},
		{
			DelegateOwnerWallet: "0x0C32BE6328578E99b6F06E0e7A6B385EB8FC13d1",
			Endpoint:            "https://creatornode3.audius.co",
			OwnerWallet:         "0xe5b256d302ea2f4e04b8f3bfd8695ade147ab68d",
		},
		{
			DelegateOwnerWallet: "0xBfdE9a7DD3620CB6428463E9A9e9932B4d10fdc5",
			Endpoint:            "https://audius-content-1.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0x675086B880260D217963cF14F503272AEb44b2E9",
			Endpoint:            "https://creatornode.audius.prod-eks-ap-northeast-1.staked.cloud",
			OwnerWallet:         "0x8C860adb28CA8A33dB5571536BFCF7D6522181e5",
		},
		{
			DelegateOwnerWallet: "0x6444212FFc23a4CcF7460f8Fe6D0e6074db59036",
			Endpoint:            "https://audius-content-2.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0xECEDCaABecb40ef4bE733BA47FaD612aeA1F396F",
			Endpoint:            "https://audius-content-3.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0x08fEF3884Db16E2E6211272cdC9Eee68E8b63b09",
			Endpoint:            "https://audius-content-4.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0x10fF8197f2e94eF880d940D2414E0A14983c3bFE",
			Endpoint:            "https://audius-content-5.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0xC23Ee959E0B22a9B0F5dF18D7e7875cA4B6c4236",
			Endpoint:            "https://creatornode.audius1.prod-eks-ap-northeast-1.staked.cloud",
			OwnerWallet:         "0x8C860adb28CA8A33dB5571536BFCF7D6522181e5",
		},
		{
			DelegateOwnerWallet: "0x51a5575dc04c1f5f2e39390d090aaf78554F5f7B",
			Endpoint:            "https://creatornode.audius2.prod-eks-ap-northeast-1.staked.cloud",
			OwnerWallet:         "0x8C860adb28CA8A33dB5571536BFCF7D6522181e5",
		},
		{
			DelegateOwnerWallet: "0xe0b56BAe2276E016d3DB314Dd7374e596B0457ac",
			Endpoint:            "https://creatornode.audius3.prod-eks-ap-northeast-1.staked.cloud",
			OwnerWallet:         "0x8C860adb28CA8A33dB5571536BFCF7D6522181e5",
		},
		{
			DelegateOwnerWallet: "0x68a4Bd6b4177ffB025AF9844cBE4Fe31348AEE1D",
			Endpoint:            "https://audius-content-6.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0xf45a6DBf3ce0201F4012a19b1fB04D4f05B53a37",
			Endpoint:            "https://audius-content-7.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0x9708Fb04DeA029212126255B311a21F1F884cCB4",
			Endpoint:            "https://audius-content-8.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0x7c34c9709ed69513D55dF2020e799DA44fC52E6e",
			Endpoint:            "https://audius-content-9.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0xff753331CEa586DD5B23bd21222a3c902909F2dd",
			Endpoint:            "https://audius-content-10.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0xC9721F892BcC8822eb34237E875BE93904f11073",
			Endpoint:            "https://audius-content-11.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0x807C0fba7405aeb8b6a37A974df6259C6aB9bB1e",
			Endpoint:            "https://blockdaemon-audius-content-01.bdnodes.net",
			OwnerWallet:         "0x091D2190e93A9C09f99dA05ec3F82ef5D8aa4a07",
		},
		{
			DelegateOwnerWallet: "0xCEb6a23d6132Cfe329b3c8E3c45f9DDc28A62Bd4",
			Endpoint:            "https://audius-content-1.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0x2e9e7A4e35C3136fB651a0dBF8f91c9f5C27BBf7",
			Endpoint:            "https://audius-content-2.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0x742da6cAc2782FeA961bB7B9150a048F5167D1e1",
			Endpoint:            "https://audius-content-3.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0xcbb0cE7481685587b0988195Ff0cD6AA1A701657",
			Endpoint:            "https://audius-content-4.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0xFec4708155277D35d568aD6Ca322262577683584",
			Endpoint:            "https://audius-content-5.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0x3Db0E61591063310eEd22fd57E6f7F1ab2Bb538E",
			Endpoint:            "https://audius-content-6.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0xE6C00e7E8d582fD2856718a5439f1aeEB68e27E5",
			Endpoint:            "https://audius-content-7.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0x4Ad694B3fC34b3cC245aF6AA7B43C52ddD0d7AAE",
			Endpoint:            "https://blockdaemon-audius-content-02.bdnodes.net",
			OwnerWallet:         "0x091D2190e93A9C09f99dA05ec3F82ef5D8aa4a07",
		},
		{
			DelegateOwnerWallet: "0x8ea81225013719950E968DE0602c4Eca458fA9f4",
			Endpoint:            "https://blockdaemon-audius-content-03.bdnodes.net",
			OwnerWallet:         "0x091D2190e93A9C09f99dA05ec3F82ef5D8aa4a07",
		},
		{
			DelegateOwnerWallet: "0xcfFA8ACF0b04d9278eEE13928be264b2E9aaab97",
			Endpoint:            "https://blockdaemon-audius-content-04.bdnodes.net",
			OwnerWallet:         "0xEe39B44cE36384157585C19df17d9B28D5637C4D",
		},
		{
			DelegateOwnerWallet: "0xB4Ff0cab630FB05a7fcEfec9E979a968b8f4fE55",
			Endpoint:            "https://blockdaemon-audius-content-05.bdnodes.net",
			OwnerWallet:         "0xEe39B44cE36384157585C19df17d9B28D5637C4D",
		},
		{
			DelegateOwnerWallet: "0x7449da7d1548C11c481b87667EC9b2A8F20C13A0",
			Endpoint:            "https://blockdaemon-audius-content-06.bdnodes.net",
			OwnerWallet:         "0xEe39B44cE36384157585C19df17d9B28D5637C4D",
		},
		{
			DelegateOwnerWallet: "0x00B1CA1A34257860f66e742eF163Ad30bF42d075",
			Endpoint:            "https://blockdaemon-audius-content-07.bdnodes.net",
			OwnerWallet:         "0x447E3572B5511cc6ea0700e34D2443017D081d7e",
		},
		{
			DelegateOwnerWallet: "0x16650eDB44C720ea627d5a59ff0b4f74c37fe419",
			Endpoint:            "https://blockdaemon-audius-content-08.bdnodes.net",
			OwnerWallet:         "0x447E3572B5511cc6ea0700e34D2443017D081d7e",
		},
		{
			DelegateOwnerWallet: "0xD5Cfcf4149c683516239fc653D5a470F3F4A606D",
			Endpoint:            "https://blockdaemon-audius-content-09.bdnodes.net",
			OwnerWallet:         "0x447E3572B5511cc6ea0700e34D2443017D081d7e",
		},
		{
			DelegateOwnerWallet: "0xff432F81D0eb77DA5973Cf55e24A897882fdd3E6",
			Endpoint:            "https://audius-content-8.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0x8464c88502925a0076c381962F8B70b6EC892861",
			Endpoint:            "https://blockchange-audius-content-01.bdnodes.net",
			OwnerWallet:         "0x59938DF0F43DC520404e4aafDdae688a455Be870",
		},
		{
			DelegateOwnerWallet: "0x5e0D0BeDC11F0B512457f6f707A35703b1447Fb5",
			Endpoint:            "https://blockchange-audius-content-02.bdnodes.net",
			OwnerWallet:         "0x59938DF0F43DC520404e4aafDdae688a455Be870",
		},
		{
			DelegateOwnerWallet: "0xe3F1c416c3919bB2ffD78F1e38b9E81E8c80815F",
			Endpoint:            "https://blockchange-audius-content-03.bdnodes.net",
			OwnerWallet:         "0x59938DF0F43DC520404e4aafDdae688a455Be870",
		},
		{
			DelegateOwnerWallet: "0xB6f506557B2e9026743FeA6157e52F204D26690F",
			Endpoint:            "https://audius-content-9.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0x2AF4598D3CF95D8e76987c02BC8A8D71F58d49d5",
			Endpoint:            "https://audius-content-10.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0xB2684Cca5281d2bA6D9Ce66Cca215635FF2Ba466",
			Endpoint:            "https://audius-content-11.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0x28924C99822eA08bFCeDdE3a411308633948b349",
			Endpoint:            "https://audius-content-12.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0xcb23908aa0dCDef762ebEaA38391D8fFC69E6e8F",
			Endpoint:            "https://audius-content-13.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0xCbDa351492e52fdb2f0E7FBc440cA2047738b71C",
			Endpoint:            "https://audius-content-14.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0x2fE2652296c40BB22D33C6379558Bf63A25b4f9a",
			Endpoint:            "https://audius-content-15.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0x47367ED3Db5D9691d866cb09545DE7cccD571579",
			Endpoint:            "https://audius-content-16.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0xb472c555Ab9eA1D33543383d6d1F8885c97eF83A",
			Endpoint:            "https://audius-content-17.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0x4F62C17Dc54E58289354847974E1F246c8EAcf11",
			Endpoint:            "https://audius-content-18.cultur3stake.com",
			OwnerWallet:         "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990",
		},
		{
			DelegateOwnerWallet: "0x780641e157621621658F118375dc1B36Ea514d46",
			Endpoint:            "https://audius-content-12.figment.io",
			OwnerWallet:         "0x700a11aE95E34fBC769f8EAD063403987Bd0C502",
		},
		{
			DelegateOwnerWallet: "0xf9b373E223b73473C59034072263f52aEF60133B",
			Endpoint:            "https://cn0.mainnet.audiusindex.org",
			OwnerWallet:         "0x528D6Fe7dF9356C8EabEC850B0f908F53075B382",
		},
		{
			DelegateOwnerWallet: "0x9b0D01bd7F01BD6916Ba139743Ce9C524B9375Dd",
			Endpoint:            "https://cn1.mainnet.audiusindex.org",
			OwnerWallet:         "0x528D6Fe7dF9356C8EabEC850B0f908F53075B382",
		},
		{
			DelegateOwnerWallet: "0xf6e297203c0086dc229DAE17F5b61a15F42A1A00",
			Endpoint:            "https://cn2.mainnet.audiusindex.org",
			OwnerWallet:         "0x528D6Fe7dF9356C8EabEC850B0f908F53075B382",
		},
		{
			DelegateOwnerWallet: "0x24C4b2cb6eC4c87a03F66723d8750dbe98Fa3e4f",
			Endpoint:            "https://cn3.mainnet.audiusindex.org",
			OwnerWallet:         "0x528D6Fe7dF9356C8EabEC850B0f908F53075B382",
		},
		{
			DelegateOwnerWallet: "0x33a2da466B14990E0124383204b06F9196f62d8e",
			Endpoint:            "https://audius-content-13.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0x817c513C1B702eA0BdD4F8C1204C60372f715006",
			Endpoint:            "https://audius-content-14.figment.io",
			OwnerWallet:         "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0",
		},
		{
			DelegateOwnerWallet: "0x69e749266C59757dA81F8C659Be6B07ce5Bac6C9",
			Endpoint:            "https://cn4.mainnet.audiusindex.org",
			OwnerWallet:         "0x528D6Fe7dF9356C8EabEC850B0f908F53075B382",
		},
		{
			DelegateOwnerWallet: "0x0E0aF7035581C615d07372be16D99A9B64E5B2e9",
			Endpoint:            "https://audius-creator-1.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
		{
			DelegateOwnerWallet: "0x3D0dD2Cd46c2658d228769f4a394662946A28987",
			Endpoint:            "https://audius-creator-2.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
		{
			DelegateOwnerWallet: "0x292B0d5987a7DE879909C48a54f0853C211da5f3",
			Endpoint:            "https://audius-creator-3.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
		{
			DelegateOwnerWallet: "0xA815f8108C2772D24D7DCB866c861148f043224D",
			Endpoint:            "https://audius-creator-4.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
		{
			DelegateOwnerWallet: "0x65Fe5BEf65A0E0b0520d6beE7767ea6Da7f792f6",
			Endpoint:            "https://audius-creator-5.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
		{
			DelegateOwnerWallet: "0x19B026B0f0Dbf619DBf8C4Efb0190308ace56366",
			Endpoint:            "https://audius-creator-6.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
		{
			DelegateOwnerWallet: "0xc69F344FCDbc9D747559c968562f682ABfBa442C",
			Endpoint:            "https://creatornode.audius8.prod-eks-ap-northeast-1.staked.cloud",
			OwnerWallet:         "0x8C860adb28CA8A33dB5571536BFCF7D6522181e5",
		},
		{
			DelegateOwnerWallet: "0x0D16f8bBfFF114B1a525Bf8b8d98ED177FA74AD3",
			Endpoint:            "https://cn1.stuffisup.com",
			OwnerWallet:         "0x3E2Cd6d498b412Da182Ef25837F72355f8918BE9",
		},
		{
			DelegateOwnerWallet: "0x159200F84c2cF000b3A014cD4D8244500CCc36ca",
			Endpoint:            "https://audius-cn1.tikilabs.com",
			OwnerWallet:         "0xe4882D9A38A2A1fc652996719AF0fb15CB968d0a",
		},
		{
			DelegateOwnerWallet: "0x720758adEa33433833c14e2516fA421261D0875e",
			Endpoint:            "https://audius-creator-7.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
		{
			DelegateOwnerWallet: "0x44955AD360652c302644F564B42D1458C584A4ec",
			Endpoint:            "https://cn1.shakespearetech.com",
			OwnerWallet:         "0x45FC5529a17f0c5285173Ad08359C53Fa8a674b4",
		},
		{
			DelegateOwnerWallet: "0x68835714d9c208f9d6F4953F0555507e492fd898",
			Endpoint:            "https://cn2.shakespearetech.com",
			OwnerWallet:         "0x45FC5529a17f0c5285173Ad08359C53Fa8a674b4",
		},
		{
			DelegateOwnerWallet: "0x7162Ee2b7F0cB9651fd2FA2838B0CAF225B2a8D3",
			Endpoint:            "https://cn3.shakespearetech.com",
			OwnerWallet:         "0x45FC5529a17f0c5285173Ad08359C53Fa8a674b4",
		},
		{
			DelegateOwnerWallet: "0x078842E88B82e6a69549043269AE3aADD5581105",
			Endpoint:            "https://audius-creator-8.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
		{
			DelegateOwnerWallet: "0x2DfC8152eF49e91b83638ad2bd0D2F9efC6f65b5",
			Endpoint:            "https://audius-creator-9.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
		{
			DelegateOwnerWallet: "0x97BcBFA8289731d694440795094E831599Ab7A11",
			Endpoint:            "https://audius-creator-10.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
		{
			DelegateOwnerWallet: "0xfe38c5Ea3579c9333fE302414fe1895F7a320beF",
			Endpoint:            "https://audius-creator-11.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
		{
			DelegateOwnerWallet: "0x8C78ef541135e2cb037f91109fb8EE780fa4709d",
			Endpoint:            "https://audius-creator-12.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
		{
			DelegateOwnerWallet: "0x75D2269D18C59CC2ED00a63a40367AC495E3F330",
			Endpoint:            "https://audius-creator-13.theblueprint.xyz",
			OwnerWallet:         "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B",
		},
	}
	StageNodes = []Node{
		{
			DelegateOwnerWallet: "0x8fcFA10Bd3808570987dbb5B1EF4AB74400FbfDA",
			Endpoint:            "https://discoveryprovider.staging.audius.co",
			OwnerWallet:         "0x8fcFA10Bd3808570987dbb5B1EF4AB74400FbfDA",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x5E98cBEEAA2aCEDEc0833AC3D1634E2A7aE0f3c2",
			Endpoint:            "https://discoveryprovider2.staging.audius.co",
			OwnerWallet:         "0x5E98cBEEAA2aCEDEc0833AC3D1634E2A7aE0f3c2",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0xf7C96916bd37Ad76D4EEDd6536B81c29706C8056",
			Endpoint:            "https://discoveryprovider3.staging.audius.co",
			OwnerWallet:         "0xf7C96916bd37Ad76D4EEDd6536B81c29706C8056",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x8311f59B72522e728231dC60226359A51878F9A1",
			Endpoint:            "https://discoveryprovider5.staging.audius.co",
			OwnerWallet:         "0x8311f59B72522e728231dC60226359A51878F9A1",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x6b52969934076318863243fb92E9C4b3A08267b5",
			Endpoint:            "https://creatornode12.staging.audius.co",
			OwnerWallet:         "0x5E98cBEEAA2aCEDEc0833AC3D1634E2A7aE0f3c2",
		},
		{
			DelegateOwnerWallet: "0xDC2BDF1F23381CA2eC9e9c70D4FD96CD8645D090",
			Endpoint:            "https://creatornode5.staging.audius.co",
			OwnerWallet:         "0xf7C96916bd37Ad76D4EEDd6536B81c29706C8056",
		},
		{
			DelegateOwnerWallet: "0x68039d001D87E7A5E6B06fe0825EA7871C1Cd6C2",
			Endpoint:            "https://creatornode6.staging.audius.co",
			OwnerWallet:         "0xf7C96916bd37Ad76D4EEDd6536B81c29706C8056",
		},
		{
			DelegateOwnerWallet: "0x1F8e7aF58086992Ef4c4fc0371446974BBbC0D9F",
			Endpoint:            "https://creatornode7.staging.audius.co",
			OwnerWallet:         "0x5E98cBEEAA2aCEDEc0833AC3D1634E2A7aE0f3c2",
		},
		{
			DelegateOwnerWallet: "0x140eD283b33be2145ed7d9d15f1fE7bF1E0B2Ac3",
			Endpoint:            "https://creatornode9.staging.audius.co",
			OwnerWallet:         "0x5E98cBEEAA2aCEDEc0833AC3D1634E2A7aE0f3c2",
		},
		{
			DelegateOwnerWallet: "0x4c88d2c0f4c4586b41621aD6e98882ae904B98f6",
			Endpoint:            "https://creatornode11.staging.audius.co",
			OwnerWallet:         "0x5E98cBEEAA2aCEDEc0833AC3D1634E2A7aE0f3c2",
		},
	}
	DevNodes = []Node{
		{
			DelegateOwnerWallet: "0x73EB6d82CFB20bA669e9c178b718d770C49BB52f",
			Endpoint:            "http://audius-protocol-discovery-provider-1",
			OwnerWallet:         "0x73EB6d82CFB20bA669e9c178b718d770C49BB52f",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x0D38e653eC28bdea5A2296fD5940aaB2D0B8875c",
			Endpoint:            "http://audius-protocol-creator-node-1",
			OwnerWallet:         "0x0D38e653eC28bdea5A2296fD5940aaB2D0B8875c",
		},
		{
			DelegateOwnerWallet: "0x1B569e8f1246907518Ff3386D523dcF373e769B6",
			Endpoint:            "http://audius-protocol-creator-node-2",
			OwnerWallet:         "0x1B569e8f1246907518Ff3386D523dcF373e769B6",
		},
		{
			DelegateOwnerWallet: "0xCBB025e7933FADfc7C830AE520Fb2FD6D28c1065",
			Endpoint:            "http://audius-protocol-creator-node-3",
			OwnerWallet:         "0xCBB025e7933FADfc7C830AE520Fb2FD6D28c1065",
		},
	}
)
