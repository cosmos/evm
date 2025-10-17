package clients

import (
	"math/big"
)

/**
# ---------------- dev mnemonics source ----------------
# dev0 address 0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101 | cosmos1cml96vmptgw99syqrrz8az79xer2pcgp84pdun
# dev0's private key: 0x88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305 # gitleaks:allow

# dev1 address 0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17 | cosmos1jcltmuhplrdcwp7stlr4hlhlhgd4htqh3a79sq
# dev1's private key: 0x741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544 # gitleaks:allow

# dev2 address 0x40a0cb1C63e026A81B55EE1308586E21eec1eFa9 | cosmos1gzsvk8rruqn2sx64acfsskrwy8hvrmafqkaze8
# dev2's private key: 0x3b7955d25189c99a7468192fcbc6429205c158834053ebe3f78f4512ab432db9 # gitleaks:allow

# dev3 address 0x498B5AeC5D439b733dC2F58AB489783A23FB26dA | cosmos1fx944mzagwdhx0wz7k9tfztc8g3lkfk6rrgv6l
# dev3's private key: 0x8a36c69d940a92fcea94b36d0f2928c7a0ee19a90073eda769693298dfa9603b # gitleaks:allow

# dev4 address 0x635EA5252CE2D882C7962cF0e055769fc5ba0aa1 | cosmos1z8rzxyclp9kgftvqmv3dmh7ysycqgndcdkcncc
# dev4's private key: 0xd0fcf593b79b2eab47571cd692015995205a8af4269427fcbfbe807efd185f4a # gitleaks:allow

# dev5 address 0xaBF22da25A35428273C0eCacb58Ae398EB51F9d1 | cosmos15m42rzmfa0l66rn54nat4xxgk8x2n2f7hmjhzy
# dev5's private key: 0x2d8dbe73fa0eac0d984bf8f10782c778a57db54f52b04a179fa752a5d109e939 # gitleaks:allow

# dev6 address 0xab37A57eCB56724cb4D61971d5e373C8721b9b24 | cosmos1pgn0e5hnjksx64etw2l8hencqmsat0g7q9jdxq
# dev6's private key: 0xe6a17482e71865e5c4bf1fd3437ce6a1db3a03d87ac4a20e8a674c5bf9d69fb5 # gitleaks:allow

# dev7 address 0xeA439F15B06B19462806dBC4e87C6539608968e3 | cosmos1u7t0upq53qs9zd7fxeahnd3jfu0yvkvdsg22nl
# dev7's private key: 0x9918d00eb9cfef26ae0a6e716274c1e200d8440e5af17cb24f6447c9f6476559 # gitleaks:allow

# dev8 address 0xf698667065F0aC856f87244116FC64945062E9aF | cosmos1wmq0v7ljagmc24z8zj7rhne2tfm5twap26zvfv
# dev8's private key: 0x4eb3b8ff87ed40122bf0471becc68b96f63c2942b54e91acd779f39abea48887 # gitleaks:allow

# dev9 address 0x361fFf6aF569b4bbA96089455C030D4Fc6038Cb5 | cosmos1v5l86j9w6de9we2snhv5vlanuha03rppduv9h3
# dev9's private key: 0xa056f4cf6ec8fbd489dd1e84b1afd43e56f1f19d83917557c8a760767d91d7e1 # gitleaks:allow
*/

const (
	ChainID    = "local-4221"
	EVMChainID = 4221

	Acc0PrivKey = "88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305"
	Acc1PrivKey = "741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544"
	Acc2PrivKey = "3b7955d25189c99a7468192fcbc6429205c158834053ebe3f78f4512ab432db9"
	Acc3PrivKey = "8a36c69d940a92fcea94b36d0f2928c7a0ee19a90073eda769693298dfa9603b"
	Acc4PrivKey = "d0fcf593b79b2eab47571cd692015995205a8af4269427fcbfbe807efd185f4a"
	Acc5PrivKey = "2d8dbe73fa0eac0d984bf8f10782c778a57db54f52b04a179fa752a5d109e939"
	Acc6PrivKey = "e6a17482e71865e5c4bf1fd3437ce6a1db3a03d87ac4a20e8a674c5bf9d69fb5"
	Acc7PrivKey = "9918d00eb9cfef26ae0a6e716274c1e200d8440e5af17cb24f6447c9f6476559"
	Acc8PrivKey = "4eb3b8ff87ed40122bf0471becc68b96f63c2942b54e91acd779f39abea48887"
	Acc9PrivKey = "a056f4cf6ec8fbd489dd1e84b1afd43e56f1f19d83917557c8a760767d91d7e1"

	JsonRPCUrl0 = "http://127.0.0.1:8545"
	JsonRPCUrl1 = "http://127.0.0.1:8555"
	JsonRPCUrl2 = "http://127.0.0.1:8565"
	JsonRPCUrl3 = "http://127.0.0.1:8575"

	NodeRPCUrl0 = "http://127.0.0.1:26657"
	NodeRPCUrl1 = "http://127.0.0.1:26658"
	NodeRPCUrl2 = "http://127.0.0.1:26659"
	NodeRPCUrl3 = "http://127.0.0.1:26660"
)

type Config struct {
	ChainID     string
	EVMChainID  *big.Int
	PrivKeys    []string
	JsonRPCUrls []string
	NodeRPCUrls []string
}

// NewConfig creates a new Config instance.
func NewConfig() (*Config, error) {

	// private keys of test accounts
	privKeys := []string{
		Acc0PrivKey,
		Acc1PrivKey,
		Acc2PrivKey,
		Acc3PrivKey,
		Acc4PrivKey,
		Acc5PrivKey,
		Acc6PrivKey,
		Acc7PrivKey,
		Acc8PrivKey,
		Acc9PrivKey,
	}

	// jsonrpc urls of testnet nodes
	jsonRPCUrls := []string{JsonRPCUrl0, JsonRPCUrl1, JsonRPCUrl2, JsonRPCUrl3}

	// rpc urls of test nodes
	nodeRPCUrls := []string{NodeRPCUrl0, NodeRPCUrl1, NodeRPCUrl2, NodeRPCUrl3}

	return &Config{
		ChainID:     ChainID,
		EVMChainID:  big.NewInt(EVMChainID),
		PrivKeys:    privKeys,
		JsonRPCUrls: jsonRPCUrls,
		NodeRPCUrls: nodeRPCUrls,
	}, nil
}
