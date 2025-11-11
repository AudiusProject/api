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
		// discovery-node entries (storage disabled)
		{DelegateOwnerWallet: "0x7db3789e5E2154569e802945ECF2cC92e0994841", Endpoint: "https://audius-metadata-1.figment.io", OwnerWallet: "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x67154199E79bEcd2A1f34f89d6AF962CF9863945", Endpoint: "https://dn1.matterlightblooming.xyz", OwnerWallet: "0xb5F5280e275eCa21f167d870d054b90C9C7e6669", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xd8091A289BEf13b5407082Bb66000ccA47e7e34C", Endpoint: "https://audius-discovery-9.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xb4c7895739062A54F33998D65eF90afb3689d765", Endpoint: "https://discovery.grassfed.network", OwnerWallet: "0x57B1d346CDe1d2fA740F310b0d358d07d7c49547", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xE83699015c8eb793A0678eA7dC398ac58f7928c4", Endpoint: "https://audius-nodes.com", OwnerWallet: "0xE83699015c8eb793A0678eA7dC398ac58f7928c4", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x8449169096550905B903b6803FB3b64285112603", Endpoint: "https://audius-discovery-3.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x048cFedf907c4C9dDD11ff882380906E78E84BbE", Endpoint: "https://blockchange-audius-discovery-03.bdnodes.net", OwnerWallet: "0x59938DF0F43DC520404e4aafDdae688a455Be870", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x16e8DF288BF5DcD507615A715A2a6155F149a865", Endpoint: "https://audius-discovery-4.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xE77C7679ED77b175F935755EEb3a421635AF07EC", Endpoint: "https://audius-discovery-1.altego.net", OwnerWallet: "0xA9cB9d043d4841dE83C70556FF0Bd4949C15b5Eb", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xf7441A14A31199744Bf8e7b79405c5446C120D0f", Endpoint: "https://audius-metadata-4.figment.io", OwnerWallet: "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x2416D78b3cc41467c22578dEE7CA90450EB6526e", Endpoint: "https://blockdaemon-audius-discovery-03.bdnodes.net", OwnerWallet: "0xEe39B44cE36384157585C19df17d9B28D5637C4D", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xE019F1Ad9803cfC83e11D37Da442c9Dc8D8d82a6", Endpoint: "https://audius-metadata-3.figment.io", OwnerWallet: "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xCF3f359BfdE7bcAfE4bc058B6DFae51aBe204aB4", Endpoint: "https://audius-discovery-2.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x16e8DF288BF5DcD507615A715A2a6155F149a865", Endpoint: "https://audius-discovery-4.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xE77C7679ED77b175F935755EEb3a421635AF07EC", Endpoint: "https://audius-discovery-1.altego.net", OwnerWallet: "0xA9cB9d043d4841dE83C70556FF0Bd4949C15b5Eb", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xAB30eF276ADC2bE22CE58d75B4F4009173A73676", Endpoint: "https://blockchange-audius-discovery-02.bdnodes.net", OwnerWallet: "0x59938DF0F43DC520404e4aafDdae688a455Be870", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xA9cB9d043d4841dE83C70556FF0Bd4949C15b5Eb", Endpoint: "https://audius-discovery-2.altego.net", OwnerWallet: "0xA9cB9d043d4841dE83C70556FF0Bd4949C15b5Eb", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x422541273087beC833c57D3c15B9e17F919bFB1F", Endpoint: "https://dn2.monophonic.digital", OwnerWallet: "0x6470Daf3bd32f5014512bCdF0D02232f5640a5BD", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xE83699015c8eb793A0678eA7dC398ac58f7928c4", Endpoint: "https://audius-nodes.com", OwnerWallet: "0xE83699015c8eb793A0678eA7dC398ac58f7928c4", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x8464c88502925a0076c381962F8B70b6EC892861", Endpoint: "https://blockdaemon-audius-discovery-08.bdnodes.net", OwnerWallet: "0x091D2190e93A9C09f99dA05ec3F82ef5D8aa4a07", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x69cfDc1AB75384f077E4E48cf0d6483C8fB9B8A2", Endpoint: "https://audius-metadata-5.figment.io", OwnerWallet: "0x700a11aE95E34fBC769f8EAD063403987Bd0C502", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xAA29e93f4008D977078957D8f041AEAeF7e1eeBc", Endpoint: "https://dn1.stuffisup.com", OwnerWallet: "0x3E2Cd6d498b412Da182Ef25837F72355f8918BE9", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x42D35a2f33ba468fA9eB6FFEA4b975F182957556", Endpoint: "https://dn1.nodeoperator.io", OwnerWallet: "0x858e345E9DC681357ecd44bA285e04180c481fF6", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x6CAA3671162bC259094Ea4451d0d16792431C37a", Endpoint: "https://discovery-au-02.audius.openplayer.org", OwnerWallet: "0x55fc79f85eEc693A65f79DB463dc3E6831364Bce", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xbD0548Ce77e69CE22Af591A4155162A08fDDEC3d", Endpoint: "https://blockdaemon-audius-discovery-04.bdnodes.net", OwnerWallet: "0xEe39B44cE36384157585C19df17d9B28D5637C4D", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xD207D8Eb95aA5b2595cF3EEA14308EB61A36ad21", Endpoint: "https://blockchange-audius-discovery-01.bdnodes.net", OwnerWallet: "0x59938DF0F43DC520404e4aafDdae688a455Be870", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x319211E15876156BD992dd047587d0cd7b88Be77", Endpoint: "https://blockchange-audius-discovery-05.bdnodes.net", OwnerWallet: "0x59938DF0F43DC520404e4aafDdae688a455Be870", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xFD005a90cc8AF8B766F9F9cD95ee91921cC9286d", Endpoint: "https://audius-disc1.nodemagic.com", OwnerWallet: "0xf13612C7d6E31636eCC2b670d6F8a3CC50f68A48", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x5cA0d3a6590074B9fF31972824178f69e8dAB547", Endpoint: "https://audius-disc2.nodemagic.com", OwnerWallet: "0xf13612C7d6E31636eCC2b670d6F8a3CC50f68A48", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0xE77C7679ED77b175F935755EEb3a421635AF07EC", Endpoint: "https://audius-discovery-1.altego.net", OwnerWallet: "0xA9cB9d043d4841dE83C70556FF0Bd4949C15b5Eb", IsStorageDisabled: true},
		{DelegateOwnerWallet: "0x57B1d346CDe1d2fA740F310b0d358d07d7c49547", Endpoint: "https://discovery.grassfed.network", OwnerWallet: "0x57B1d346CDe1d2fA740F310b0d358d07d7c49547", IsStorageDisabled: true},
		// validator entries (no storage disabled)
		{DelegateOwnerWallet: "0xf686647E3737d595C60c6DE2f5F90463542FE439", Endpoint: "https://creatornode2.audius.co", OwnerWallet: "0xe5b256d302ea2f4e04B8F3bfD8695aDe147aB68d"},
		{DelegateOwnerWallet: "0x8162684B7a004AF41de09Bfd3bE6Ef53d64158F5", Endpoint: "https://audius-discovery-4.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0xB98a729444E262ec9AC5F1539b68e69aEC37Ef2C", Endpoint: "https://audius-discovery-2.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x222f0e1eC3a08826aE8004b0aC1Dcf3334A1b0F0", Endpoint: "https://audius-discovery-16.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0xEC44A3E06241D24A872dC235bAd4EA1ACC9e0A97", Endpoint: "https://audius-discovery-11.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x2aBFC8CA43D046181AaC6281926D710a9601EFE2", Endpoint: "https://audius-discovery-5.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x0C32BE6328578E99b6F06E0e7A6B385EB8FC13d1", Endpoint: "https://creatornode3.audius.co", OwnerWallet: "0xe5b256d302ea2f4e04B8F3bfD8695aDe147aB68d"},
		{DelegateOwnerWallet: "0x241Da559e97d2e76f37F7144a04623849Fa576ff", Endpoint: "https://audius-discovery-10.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0xCD3eeE4f50fb35504Aed4c0Ca5841eb174428D87", Endpoint: "https://audius-discovery-17.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0xb03478a9e8fB1516AA91b272Afc7422b1c71D837", Endpoint: "https://audius-discovery-7.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0xeEB73B487f8B4b23B757d9d738Bfc48fD0cc2606", Endpoint: "https://audius-discovery-3.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x493BC4f3E1C29B419E4C62617ecC3d0238F7AA99", Endpoint: "https://audius-discovery-14.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x73956a079133b6fA0Ecd2777112404956E9d9272", Endpoint: "https://audius-discovery-18.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x7DbfF3c39acC8fBb660efFBdaeA4A7172dd45ae7", Endpoint: "https://audius-discovery-12.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x159200F84c2cF000b3A014cD4D8244500CCc36ca", Endpoint: "https://audius-cn1.tikilabs.com", OwnerWallet: "0xe4882D9A38A2A1fc652996719AF0fb15CB968d0a"},
		{DelegateOwnerWallet: "0xc8d0C29B6d540295e8fc8ac72456F2f4D41088c8", Endpoint: "https://creatornode.audius.co", OwnerWallet: "0xe5b256d302ea2f4e04B8F3bfD8695aDe147aB68d"},
		{DelegateOwnerWallet: "0xae5D0507b6653589A03ae5becb35EB0C160e7131", Endpoint: "https://audius.rickyrombo.com", OwnerWallet: "0x923EC9976bfEcFd0E8b7fEeaC9115F740f8ddB00"},
		{DelegateOwnerWallet: "0x627d23D17a3eAaDB1D3823e73Ab80D474023Acab", Endpoint: "https://audius.bragi.cc", OwnerWallet: "0xC88C8F9a15453c7D8Ea83120Af54cc4C40EC094a"},
		{DelegateOwnerWallet: "0x3f0cbB987b82f620dF313A74999f025E03A11a30", Endpoint: "https://audius-discovery-1.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x9D0a106E6cE7643DF87914c7387b3d864bfA152B", Endpoint: "https://audius-discovery-8.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x650CE30dD821901e4790B9912fa502878Eb6E18A", Endpoint: "https://audius-discovery-6.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x32Af5552adbCc4CbFe1C9C6B3F0117DB4CB0Cf08", Endpoint: "https://audius-discovery-15.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0xe72F2722AE881469835C0B1B16E310D4E8Afdc74", Endpoint: "https://audius-discovery-13.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		// content-node entries (no storage disabled)
		{DelegateOwnerWallet: "0x6444212FFc23a4CcF7460f8Fe6D0e6074db59036", Endpoint: "https://audius-content-2.figment.io", OwnerWallet: "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0"},
		{DelegateOwnerWallet: "0x75D2269D18C59CC2ED00a63a40367AC495E3F330", Endpoint: "https://audius-creator-13.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B"},
		{DelegateOwnerWallet: "0xBfdE9a7DD3620CB6428463E9A9e9932B4d10fdc5", Endpoint: "https://audius-content-1.figment.io", OwnerWallet: "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0"},
		{DelegateOwnerWallet: "0x675086B880260D217963cF14F503272AEb44b2E9", Endpoint: "https://creatornode.audius.prod-eks-ap-northeast-1.staked.cloud", OwnerWallet: "0x8C860adb28CA8A33dB5571536BFCF7D6522181e5"},
		{DelegateOwnerWallet: "0x9708Fb04DeA029212126255B311a21F1F884cCB4", Endpoint: "https://audius-content-8.figment.io", OwnerWallet: "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0"},
		{DelegateOwnerWallet: "0xECEDCaABecb40ef4bE733BA47FaD612aeA1F396F", Endpoint: "https://audius-content-3.figment.io", OwnerWallet: "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0"},
		{DelegateOwnerWallet: "0xC9721F892BcC8822eb34237E875BE93904f11073", Endpoint: "https://audius-content-11.figment.io", OwnerWallet: "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0"},
		{DelegateOwnerWallet: "0x3Db0E61591063310eEd22fd57E6f7F1ab2Bb538E", Endpoint: "https://audius-content-6.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x00B1CA1A34257860f66e742eF163Ad30bF42d075", Endpoint: "https://blockdaemon-audius-content-07.bdnodes.net", OwnerWallet: "0x447E3572B5511cc6ea0700e34D2443017D081d7e"},
		{DelegateOwnerWallet: "0x4Ad694B3fC34b3cC245aF6AA7B43C52ddD0d7AAE", Endpoint: "https://blockdaemon-audius-content-02.bdnodes.net", OwnerWallet: "0x091D2190e93A9C09f99dA05ec3F82ef5D8aa4a07"},
		{DelegateOwnerWallet: "0x10fF8197f2e94eF880d940D2414E0A14983c3bFE", Endpoint: "https://audius-content-5.figment.io", OwnerWallet: "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0"},
		{DelegateOwnerWallet: "0x5e0D0BeDC11F0B512457f6f707A35703b1447Fb5", Endpoint: "https://blockchange-audius-content-02.bdnodes.net", OwnerWallet: "0x59938DF0F43DC520404e4aafDdae688a455Be870"},
		{DelegateOwnerWallet: "0x780641e157621621658F118375dc1B36Ea514d46", Endpoint: "https://audius-content-12.figment.io", OwnerWallet: "0x700a11aE95E34fBC769f8EAD063403987Bd0C502"},
		{DelegateOwnerWallet: "0x2fE2652296c40BB22D33C6379558Bf63A25b4f9a", Endpoint: "https://audius-content-15.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x817c513C1B702eA0BdD4F8C1204C60372f715006", Endpoint: "https://audius-content-14.figment.io", OwnerWallet: "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0"},
		{DelegateOwnerWallet: "0x33a2da466B14990E0124383204b06F9196f62d8e", Endpoint: "https://audius-content-13.figment.io", OwnerWallet: "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0"},
		{DelegateOwnerWallet: "0x65Fe5BEf65A0E0b0520d6beE7767ea6Da7f792f6", Endpoint: "https://audius-creator-5.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B"},
		{DelegateOwnerWallet: "0xe0b56BAe2276E016d3DB314Dd7374e596B0457ac", Endpoint: "https://creatornode.audius3.prod-eks-ap-northeast-1.staked.cloud", OwnerWallet: "0x8C860adb28CA8A33dB5571536BFCF7D6522181e5"},
		{DelegateOwnerWallet: "0xCbDa351492e52fdb2f0E7FBc440cA2047738b71C", Endpoint: "https://audius-content-14.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x0E0aF7035581C615d07372be16D99A9B64E5B2e9", Endpoint: "https://audius-creator-1.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B"},
		{DelegateOwnerWallet: "0x8ea81225013719950E968DE0602c4Eca458fA9f4", Endpoint: "https://blockdaemon-audius-content-03.bdnodes.net", OwnerWallet: "0x091D2190e93A9C09f99dA05ec3F82ef5D8aa4a07"},
		{DelegateOwnerWallet: "0xcfFA8ACF0b04d9278eEE13928be264b2E9aaab97", Endpoint: "https://blockdaemon-audius-content-04.bdnodes.net", OwnerWallet: "0xEe39B44cE36384157585C19df17d9B28D5637C4D"},
		{DelegateOwnerWallet: "0xB4Ff0cab630FB05a7fcEfec9E979a968b8f4fE55", Endpoint: "https://blockdaemon-audius-content-05.bdnodes.net", OwnerWallet: "0xEe39B44cE36384157585C19df17d9B28D5637C4D"},
		{DelegateOwnerWallet: "0x7449da7d1548C11c481b87667EC9b2A8F20C13A0", Endpoint: "https://blockdaemon-audius-content-06.bdnodes.net", OwnerWallet: "0xEe39B44cE36384157585C19df17d9B28D5637C4D"},
		{DelegateOwnerWallet: "0x16650eDB44C720ea627d5a59ff0b4f74c37fe419", Endpoint: "https://blockdaemon-audius-content-08.bdnodes.net", OwnerWallet: "0x447E3572B5511cc6ea0700e34D2443017D081d7e"},
		{DelegateOwnerWallet: "0xD5Cfcf4149c683516239fc653D5a470F3F4A606D", Endpoint: "https://blockdaemon-audius-content-09.bdnodes.net", OwnerWallet: "0x447E3572B5511cc6ea0700e34D2443017D081d7e"},
		{DelegateOwnerWallet: "0xff753331CEa586DD5B23bd21222a3c902909F2dd", Endpoint: "https://audius-content-10.figment.io", OwnerWallet: "0xc1f351FE81dFAcB3541e59177AC71Ed237BD15D0"},
		{DelegateOwnerWallet: "0xCEb6a23d6132Cfe329b3c8E3c45f9DDc28A62Bd4", Endpoint: "https://audius-content-1.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x2e9e7A4e35C3136fB651a0dBF8f91c9f5C27BBf7", Endpoint: "https://audius-content-2.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x742da6cAc2782FeA961bB7B9150a048F5167D1e1", Endpoint: "https://audius-content-3.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0xcbb0cE7481685587b0988195Ff0cD6AA1A701657", Endpoint: "https://audius-content-4.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0xFec4708155277D35d568aD6Ca322262577683584", Endpoint: "https://audius-content-5.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x3Db0E61591063310eEd22fd57E6f7F1ab2Bb538E", Endpoint: "https://audius-content-6.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0xE6C00e7E8d582fD2856718a5439f1aeEB68e27E5", Endpoint: "https://audius-content-7.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x5e0D0BeDC11F0B512457f6f707A35703b1447Fb5", Endpoint: "https://blockchange-audius-content-02.bdnodes.net", OwnerWallet: "0x59938DF0F43DC520404e4aafDdae688a455Be870"},
		{DelegateOwnerWallet: "0xe3F1c416c3919bB2ffD78F1e38b9E81E8c80815F", Endpoint: "https://blockchange-audius-content-03.bdnodes.net", OwnerWallet: "0x59938DF0F43DC520404e4aafDdae688a455Be870"},
		{DelegateOwnerWallet: "0xB6f506557B2e9026743FeA6157e52F204D26690F", Endpoint: "https://audius-content-9.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x2AF4598D3CF95D8e76987c02BC8A8D71F58d49d5", Endpoint: "https://audius-content-10.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0xB2684Cca5281d2bA6D9Ce66Cca215635FF2Ba466", Endpoint: "https://audius-content-11.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x28924C99822eA08bFCeDdE3a411308633948b349", Endpoint: "https://audius-content-12.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0xcb23908aa0dCDef762ebEaA38391D8fFC69E6e8F", Endpoint: "https://audius-content-13.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x28924C99822eA08bFCeDdE3a411308633948b349", Endpoint: "https://audius-content-12.cultur3stake.com", OwnerWallet: "0x2168990Cd51c7C7DdE4b16Ac4fe7dbA269768990"},
		{DelegateOwnerWallet: "0x69e749266C59757dA81F8C659Be6B07ce5Bac6C9", Endpoint: "https://cn4.mainnet.audiusindex.org", OwnerWallet: "0x528D6Fe7dF9356C8EabEC850B0f908F53075B382"},
		{DelegateOwnerWallet: "0x720758adEa33433833c14e2516fA421261D0875e", Endpoint: "https://audius-creator-7.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B"},
		{DelegateOwnerWallet: "0x44955AD360652c302644F564B42D1458C584A4ec", Endpoint: "https://cn1.shakespearetech.com", OwnerWallet: "0x45FC5529a17f0c5285173Ad08359C53Fa8a674b4"},
		{DelegateOwnerWallet: "0x68835714d9c208f9d6F4953F0555507e492fd898", Endpoint: "https://cn2.shakespearetech.com", OwnerWallet: "0x45FC5529a17f0c5285173Ad08359C53Fa8a674b4"},
		{DelegateOwnerWallet: "0x7162Ee2b7F0cB9651fd2FA2838B0CAF225B2a8D3", Endpoint: "https://cn3.shakespearetech.com", OwnerWallet: "0x45FC5529a17f0c5285173Ad08359C53Fa8a674b4"},
		{DelegateOwnerWallet: "0x078842E88B82e6a69549043269AE3aADD5581105", Endpoint: "https://audius-creator-8.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B"},
		{DelegateOwnerWallet: "0x2DfC8152eF49e91b83638ad2bd0D2F9efC6f65b5", Endpoint: "https://audius-creator-9.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B"},
		{DelegateOwnerWallet: "0x97BcBFA8289731d694440795094E831599Ab7A11", Endpoint: "https://audius-creator-10.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B"},
		{DelegateOwnerWallet: "0xfe38c5Ea3579c9333fE302414fe1895F7a320beF", Endpoint: "https://audius-creator-11.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B"},
		{DelegateOwnerWallet: "0x8C78ef541135e2cb037f91109fb8EE780fa4709d", Endpoint: "https://audius-creator-12.theblueprint.xyz", OwnerWallet: "0x68f656d19AC6d14dF209B1dd6E543b2E81d53D7B"},
	}
	StageNodes = []Node{
		{
			DelegateOwnerWallet: "0x8fcFA10Bd3808570987dbb5B1EF4AB74400FbfDA",
			Endpoint:            "https://discoveryprovider.staging.audius.co",
			OwnerWallet:         "0x8fcFA10Bd3808570987dbb5B1EF4AB74400FbfDA",
			IsStorageDisabled:   true,
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
		{
			DelegateOwnerWallet: "0x6b52969934076318863243fb92E9C4b3A08267b5",
			Endpoint:            "https://creatornode12.staging.audius.co",
			OwnerWallet:         "0x5E98cBEEAA2aCEDEc0833AC3D1634E2A7aE0f3c2",
		},
	}
	DevNodes = []Node{
		{
			DelegateOwnerWallet: "0x73EB6d82CFB20bA669e9c178b718d770C49BB52f",
			Endpoint:            "http://audius-discovery-provider-1",
			OwnerWallet:         "0x73EB6d82CFB20bA669e9c178b718d770C49BB52f",
			IsStorageDisabled:   true,
		},
		{
			DelegateOwnerWallet: "0x0D38e653eC28bdea5A2296fD5940aaB2D0B8875c",
			Endpoint:            "http://audius-creator-node-1",
			OwnerWallet:         "0x0D38e653eC28bdea5A2296fD5940aaB2D0B8875c",
		},
		{
			DelegateOwnerWallet: "0x1B569e8f1246907518Ff3386D523dcF373e769B6",
			Endpoint:            "http://audius-creator-node-2",
			OwnerWallet:         "0x1B569e8f1246907518Ff3386D523dcF373e769B6",
		},
		{
			DelegateOwnerWallet: "0xCBB025e7933FADfc7C830AE520Fb2FD6D28c1065",
			Endpoint:            "http://audius-creator-node-3",
			OwnerWallet:         "0xCBB025e7933FADfc7C830AE520Fb2FD6D28c1065",
		},
	}

	ProdUploadNodes = []string{
		"https://creatornode.audius.co",
		"https://creatornode2.audius.co",
		"https://creatornode3.audius.co",
	}
	StageUploadNodes = []string{
		"https://creatornode6.staging.audius.co",
	}
	DevUploadNodes = []string{
		"http://audius-creator-node-1",
	}
)
