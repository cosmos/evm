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
*/

const (
	ChainID    = "local-4221"
	EVMChainID = 4221

	Acc0PrivKey = "88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305"
	Acc1PrivKey = "741de4f8988ea941d3ff0287911ca4074e62b7d45c991a51186455366f10b544"
	Acc2PrivKey = "3b7955d25189c99a7468192fcbc6429205c158834053ebe3f78f4512ab432db9"
	Acc3PrivKey = "8a36c69d940a92fcea94b36d0f2928c7a0ee19a90073eda769693298dfa9603b"

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
