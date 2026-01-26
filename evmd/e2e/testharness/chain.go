package testharness

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cosmos/evm/evmd/app/params"
	"github.com/cosmos/evm/evmd/e2e/utils"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/pelletier/go-toml/v2"
	tc "github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	tcnetwork "github.com/testcontainers/testcontainers-go/network"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type Chain struct {
	Container tc.Container
	Network   *tc.DockerNetwork

	RPC               string
	WS                string
	EthClient         *ethclient.Client
	GrpcClient        *grpc.ClientConn
	EVMEncodingConfig params.EncodingConfig

	HomeDir string

	keepContainers bool
}

type Options struct {
	RepoRoot         string
	ChainID          string
	EVMChainID       uint64
	Bech32Prefix     string
	Denom            string
	DisplayDenom     string
	SenderHex        string
	SenderBech32     string
	FundSender       bool
	ConsensusTimeout map[string]string
	UnbondingTime    time.Duration
	EnableRPC        bool
	RPCAPIs          []string
	ValidatorAmount  string // e.g. "5000000000000000000astake"
	SenderAmount     string // e.g. "2000000000000000000astake"
	SelfDelegation   string // e.g. "1000000000000000000astake"
}

func NewChain() *Chain {
	keepContainers := false
	if os.Getenv("KEEP_CONTAINERS") == "true" {
		keepContainers = true
		os.Setenv("TESTCONTAINERS_RYUK_DISABLED", "true") // nolint:revive
	}
	return &Chain{HomeDir: "/data", keepContainers: keepContainers, EVMEncodingConfig: evmdEncodingConfig()}
}

func (c *Chain) Start(ctx context.Context, opts Options) error {
	// Create isolated test network (for future multi-validator support)
	net, err := tcnetwork.New(ctx, tcnetwork.WithAttachable())
	if err != nil {
		return err
	}
	c.Network = net

	req := tc.ContainerRequest{
		Name:  fmt.Sprintf("evmd-e2e-node-%s", uuid.NewString()),
		Image: "evmd:local",
		Env: map[string]string{
			"EVMD_HOME":   c.HomeDir,
			"DAEMON_HOME": c.HomeDir,
		},
		Networks: []string{c.Network.Name},
		ExposedPorts: []string{
			// TODO: seems like we could pull out some consts for the ports (used elsewhere too)
			"8545/tcp", // HTTP JSON-RPC
			"8546/tcp", // WS JSON-RPC
			"9090/tcp", // gRPC
		},
		Entrypoint: []string{"tail", "-f", "/dev/null"}, // TODO: Is this necessary?
	}
	container, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: req, Started: true})
	if err != nil {
		c.cleanup(ctx)
		return err
	}
	c.Container = container

	host, err := container.Host(ctx)
	if err != nil {
		c.cleanup(ctx)
		return err
	}

	chainID := opts.ChainID
	if chainID == "" {
		chainID = utils.TestChainID
	}

	// 4) Initialize chain home and keys
	if err := c.execArgsOK(ctx, []string{"evmd", "init", "validator", "--chain-id", chainID, "--home", c.HomeDir}); err != nil {
		c.cleanup(ctx)
		return err
	}
	if err := c.execArgsOK(ctx, []string{"evmd", "keys", "add", "val", "--keyring-backend", "test", "--home", c.HomeDir, "--output", "json"}); err != nil {
		c.cleanup(ctx)
		return err
	}

	// 5) Add genesis accounts (validator + optional sender)
	outShow, _, err := c.execArgsOut(ctx, []string{"evmd", "keys", "show", "val", "-a", "--keyring-backend", "test", "--home", c.HomeDir})
	valAddr := outShow
	if err != nil {
		c.cleanup(ctx)
		return err
	}
	valAddr = strings.TrimSpace(valAddr)

	valAmt := opts.ValidatorAmount
	if valAmt == "" {
		valAmt = "5000000000000000000" + opts.Denom // 5e18
	}
	if err := c.execArgsOK(ctx, []string{"evmd", "genesis", "add-genesis-account", valAddr, valAmt, "--home", c.HomeDir}); err != nil {
		c.cleanup(ctx)
		return err
	}

	if opts.FundSender && opts.SenderBech32 != "" {
		sendAmt := opts.SenderAmount
		if sendAmt == "" {
			sendAmt = "2000000000000000000" + opts.Denom // 2e18
		}
		if err := c.execArgsOK(ctx, []string{"evmd", "genesis", "add-genesis-account", opts.SenderBech32, sendAmt, "--home", c.HomeDir}); err != nil {
			c.cleanup(ctx)
			return err
		}
	}

	// 6) Mutate genesis (EVM denom, precompiles, WERC20 mapping, staking unbonding time)
	if err := c.mutateGenesis(ctx, opts); err != nil {
		c.cleanup(ctx)
		return err
	}

	// 7) Generate/collect gentx
	selfDel := opts.SelfDelegation
	if selfDel == "" {
		selfDel = "1000000000000000000" + opts.Denom // 1e18
	}
	if err := c.execArgsOK(ctx, []string{"evmd", "genesis", "gentx", "val", selfDel, "--chain-id", chainID, "--keyring-backend", "test", "--home", c.HomeDir}); err != nil {
		c.cleanup(ctx)
		return err
	}
	if err := c.execArgsOK(ctx, []string{"evmd", "genesis", "collect-gentxs", "--home", c.HomeDir}); err != nil {
		c.cleanup(ctx)
		return err
	}

	// Re-apply genesis mutation after gentx collection to ensure settings persist
	if err := c.mutateGenesis(ctx, opts); err != nil {
		c.cleanup(ctx)
		return err
	}

	// 8) Patch app.toml and config.toml
	if err := c.patchConfigs(ctx, opts); err != nil {
		c.cleanup(ctx)
		return err
	}

	// 9) Start evmd in background
	// include --chain-id so BaseApp matches genesis (avoids handshake chain-id mismatch)
	startCmd := fmt.Sprintf(
		"evmd start --home %s --chain-id %s --evm.evm-chain-id %d --mempool.max-txs=0 > %s/start.log 2>&1 & echo $! > %s/evmd.pid",
		c.HomeDir,
		chainID,
		opts.EVMChainID,
		c.HomeDir,
		c.HomeDir,
	)
	if err := c.execArgsOK(ctx, []string{"sh", "-c", startCmd}); err != nil {
		c.cleanup(ctx)
		return err
	}

	// 10) Resolve mapped ports and build URLs
	rpcMapped, err := container.MappedPort(ctx, "8545/tcp")
	if err != nil {
		c.cleanup(ctx)
		return err
	}
	wsMapped, err := container.MappedPort(ctx, "8546/tcp")
	if err != nil {
		c.cleanup(ctx)
		return err
	}
	grpcMapped, err := container.MappedPort(ctx, "9090/tcp")
	if err != nil {
		c.cleanup(ctx)
		return err
	}

	grpcCli, err := grpc.NewClient(fmt.Sprintf("%s:%s", host, grpcMapped.Port()), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		c.cleanup(ctx)
		return err
	}

	c.RPC = fmt.Sprintf("http://%s:%s", host, rpcMapped.Port()) // nolint:revive
	c.WS = fmt.Sprintf("ws://%s:%s", host, wsMapped.Port())     // nolint:revive
	c.GrpcClient = grpcCli

	// Initialize EthClient
	ethClient, err := ethclient.Dial(c.RPC)
	if err != nil {
		c.cleanup(ctx)
		return err
	}
	c.EthClient = ethClient

	// 11) Wait for JSON-RPC ready then a few blocks
	if err := utils.WaitForBlocks(ctx, c.EthClient, 2); err != nil {
		c.cleanup(ctx)
		return err
	}

	return nil
}

func (c *Chain) Stop(ctx context.Context) { c.cleanup(ctx) }

// ExecQuery runs `evmd query <args...> -o json --home <HomeDir>` inside the container.
func (c *Chain) ExecQuery(ctx context.Context, args ...string) (string, string, error) {
	full := append([]string{"evmd", "query"}, append(args, "-o", "json", "--home", c.HomeDir)...)
	out, exit, err := c.execArgsOut(ctx, full)
	if err != nil {
		return "", out, err
	}
	if exit != 0 {
		return "", out, fmt.Errorf("non-zero exit: %d", exit)
	}
	return out, "", nil
}

// Exec runs the provided command inside the validator container and returns combined stdout/stderr.
func (c *Chain) Exec(ctx context.Context, args ...string) (string, error) {
	out, exit, err := c.execArgsOut(ctx, args)
	if err != nil {
		return "", err
	}
	if exit != 0 {
		return out, fmt.Errorf("exec failed (%d): %s", exit, strings.TrimSpace(out))
	}
	return out, nil
}

// internals

func (c *Chain) execArgsOK(ctx context.Context, args []string) error {
	exit, reader, err := c.Container.Exec(ctx, args, tcexec.Multiplexed())
	if err != nil {
		return err
	}
	b, _ := io.ReadAll(reader)
	if exit != 0 {
		return fmt.Errorf("exec failed (%d): %s", exit, string(b))
	}
	return nil
}

func (c *Chain) execArgsOut(ctx context.Context, args []string) (string, int, error) {
	exit, reader, err := c.Container.Exec(ctx, args, tcexec.Multiplexed())
	if err != nil {
		return "", 0, err
	}
	b, _ := io.ReadAll(reader)
	return string(b), exit, nil
}

func (c *Chain) copyFileFromContainer(ctx context.Context, src string) ([]byte, error) {
	r, err := c.Container.CopyFileFromContainer(ctx, src)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// CopyFileFromContainer is an exported wrapper to allow callers outside this package to fetch files.
func (c *Chain) CopyFileFromContainer(ctx context.Context, src string) ([]byte, error) {
	return c.copyFileFromContainer(ctx, src)
}

// dumpStacks sends SIGQUIT to the evmd process to dump goroutine stacks into start.log.
// Errors are ignored to avoid interfering with test shutdown.
func (c *Chain) dumpStacks(ctx context.Context) {
	if c.Container == nil {
		return
	}
	_, _, _ = c.execArgsOut(ctx, []string{"sh", "-lc", "kill -QUIT $(cat /data/evmd.pid) 2>/dev/null || true"})
}

func (c *Chain) copyFileToContainer(ctx context.Context, dstAbs string, data []byte, mode int64) error {
	return c.Container.CopyToContainer(ctx, data, dstAbs, mode)
}

// Mutate genesis via existing utils.MutateGenesisInMemory
func (c *Chain) mutateGenesis(ctx context.Context, opts Options) error {
	path := filepath.Join(c.HomeDir, "config", "genesis.json")
	bz, err := c.copyFileFromContainer(ctx, path)
	if err != nil {
		return err
	}
	var g map[string]any
	if err := json.Unmarshal(bz, &g); err != nil {
		return err
	}
	appState := ensureMap(g, "app_state")

	// EVM params
	evm := ensureMap(appState, "evm")
	evmParams := ensureMap(evm, "params")
	evmParams["evm_denom"] = opts.Denom
	// TODO: Why do I need this?
	evmParams["active_static_precompiles"] = []any{
		"0x0000000000000000000000000000000000000100",
		"0x0000000000000000000000000000000000000400",
		"0x0000000000000000000000000000000000000800",
		"0x0000000000000000000000000000000000000801",
		"0x0000000000000000000000000000000000000802",
		"0x0000000000000000000000000000000000000803",
		"0x0000000000000000000000000000000000000804",
		"0x0000000000000000000000000000000000000805",
		"0x0000000000000000000000000000000000000806",
		"0x0000000000000000000000000000000000000807",
	}

	// Staking params: align bond_denom and shorten unbonding_time for e2e
	staking := ensureMap(appState, "staking")
	stakingParams := ensureMap(staking, "params")
	stakingParams["bond_denom"] = opts.Denom
	stakingParams["unbonding_time"] = "10s"

	// Bank genesis: set denom metadata for gas token
	bank := ensureMap(appState, "bank")

	// Standard metadata fields
	name := "evmd"
	symbol := "STAKE"
	md := banktypes.Metadata{
		Description: "Native 18-decimal denom metadata for evmd chain",
		Base:        opts.Denom,
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: opts.Denom, Exponent: 0},
			{Denom: opts.DisplayDenom, Exponent: 18},
		},
		Name:    name,
		Symbol:  symbol,
		Display: opts.DisplayDenom,
	}
	bank["denom_metadata"] = []banktypes.Metadata{md}

	out, err := json.MarshalIndent(g, "", "  ")
	if err != nil {
		return err
	}
	return c.copyFileToContainer(ctx, path, out, 0o644)
}

func ensureMap(parent map[string]any, key string) map[string]any {
	if v, ok := parent[key]; ok {
		if mm, ok := v.(map[string]any); ok {
			return mm
		}
	}
	child := make(map[string]any)
	parent[key] = child
	return child
}

func (c *Chain) patchConfigs(ctx context.Context, opts Options) error {
	// app.toml
	appPath := filepath.Join(c.HomeDir, "config", "app.toml")
	appBz, err := c.copyFileFromContainer(ctx, appPath)
	if err != nil {
		return err
	}
	app := map[string]any{}
	if err := toml.Unmarshal(appBz, &app); err != nil {
		return err
	}

	// enable Cosmos API + unsafe CORS (harmless for tests)
	if sec, ok := app["api"].(map[string]any); ok {
		sec["enable"] = true
		sec["enabled-unsafe-cors"] = true
		app["api"] = sec
	} else {
		app["api"] = map[string]any{"enable": true, "enabled-unsafe-cors": true}
	}

	// enable JSON-RPC
	if opts.EnableRPC {
		jr, ok := app["json-rpc"].(map[string]any)
		if !ok {
			jr = map[string]any{}
		}
		jr["enable"] = true
		jr["address"] = "0.0.0.0:8545"
		jr["ws-address"] = "0.0.0.0:8546"
		apis := opts.RPCAPIs
		if len(apis) == 0 {
			apis = []string{"eth", "txpool", "personal", "net", "debug", "web3"}
		}
		jr["api"] = strings.Join(apis, ",")
		app["json-rpc"] = jr
	}

	// enable gRPC and bind to all interfaces for host access
	if sec, ok := app["grpc"].(map[string]any); ok {
		sec["enable"] = true
		sec["address"] = "0.0.0.0:9090"
		app["grpc"] = sec
	} else {
		app["grpc"] = map[string]any{"enable": true, "address": "0.0.0.0:9090"}
	}

	// minimum-gas-prices for SDK (EVM txs not affected, but safe to set low)
	if opts.Denom != "" {
		app["minimum-gas-prices"] = "0" + opts.Denom
	}

	newAppBz, err := toml.Marshal(app)
	if err != nil {
		return err
	}
	if err := c.copyFileToContainer(ctx, appPath, newAppBz, 0o644); err != nil {
		return err
	}

	// config.toml
	cfgPath := filepath.Join(c.HomeDir, "config", "config.toml")
	cfgBz, err := c.copyFileFromContainer(ctx, cfgPath)
	if err != nil {
		return err
	}
	cfg := map[string]any{}
	if err := toml.Unmarshal(cfgBz, &cfg); err != nil {
		return err
	}

	if tm, ok := cfg["consensus"].(map[string]any); ok {
		for k, v := range opts.ConsensusTimeout {
			tm[k] = v
		}
		cfg["consensus"] = tm
	}
	if rpc, ok := cfg["rpc"].(map[string]any); ok {
		rpc["laddr"] = "tcp://0.0.0.0:26657"
		cfg["rpc"] = rpc
	}
	if p2p, ok := cfg["p2p"].(map[string]any); ok {
		p2p["laddr"] = "tcp://0.0.0.0:26656"
		cfg["p2p"] = p2p
	}

	newCfgBz, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	return c.copyFileToContainer(ctx, cfgPath, newCfgBz, 0o644)
}

func (c *Chain) cleanup(ctx context.Context) {
	if c.keepContainers {
		return
	}

	// Attempt to copy artifacts before tearing down the container
	if c.Container != nil {
		c.dumpStacks(ctx)
		c.saveArtifacts(ctx)

		_ = c.Container.Terminate(ctx) // errors are ignored: it is likely already terminated, we just do this for good measure
	}

	if c.Network != nil {
		if err := c.Network.Remove(ctx); err != nil {
			fmt.Printf("evmd e2e: failed removing network: %v\n", err)
		}
	}
}

// saveArtifacts copies useful runtime artifacts from the container to a host temp dir.
// Best-effort; errors are printed but do not fail cleanup.
func (c *Chain) saveArtifacts(ctx context.Context) {
	dir, err := os.MkdirTemp("", "evmd-e2e-*")
	if err != nil {
		fmt.Printf("evmd e2e: failed to create temp dir for artifacts: %v\n", err)
		return
	}

	saveArtifact := func(src, dstName string) {
		b, err := c.copyFileFromContainer(ctx, src)
		if err != nil {
			fmt.Printf("evmd e2e: failed copying %s: %v\n", src, err)
			return
		}
		if werr := os.WriteFile(filepath.Join(dir, dstName), b, 0o600); werr != nil {
			fmt.Printf("evmd e2e: failed writing %s: %v\n", dstName, werr)
		}
	}

	saveArtifact(filepath.Join(c.HomeDir, "start.log"), "start.log")
	saveArtifact(filepath.Join(c.HomeDir, "config", "genesis.json"), "genesis.json")

	fmt.Printf("evmd e2e artifacts saved: %s\n", dir)
}
