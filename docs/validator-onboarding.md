# EpixChain Validator Onboarding Guide

This comprehensive guide will help you set up and run a validator node on EpixChain for both testnet and mainnet environments.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Network Information](#network-information)
- [Hardware Requirements](#hardware-requirements)
- [Installation](#installation)
- [Testnet Setup](#testnet-setup)
- [Mainnet Setup](#mainnet-setup)
- [Validator Creation](#validator-creation)
- [Node Operations](#node-operations)
- [Monitoring & Maintenance](#monitoring--maintenance)
- [Security Best Practices](#security-best-practices)
- [Troubleshooting](#troubleshooting)

## Prerequisites

- Linux/macOS operating system
- Basic knowledge of command line operations
- Understanding of blockchain and validator concepts
- Secure server environment for mainnet operations

## Network Information

### Mainnet

- **Chain ID**: `epix_1916-1`
- **EVM Chain ID**: `1916`
- **RPC Endpoint**: `https://rpc.epix.zone`
- **API Endpoint**: `https://api.epix.zone`
- **Currency**: `EPIX` (base: `aepix`)
- **Decimals**: 18
- **Genesis File**: `https://raw.githubusercontent.com/EpixZone/EpixChain/main/artifacts/genesis/mainnet/genesis.json`

### Testnet

- **Chain ID**: `epix_1917-1`
- **EVM Chain ID**: `1917`
- **RPC Endpoint**: `https://rpc.testnet.epix.zone`
- **API Endpoint**: `https://api.testnet.epix.zone`
- **Currency**: `EPIX` (base: `aepix`)
- **Decimals**: 18
- **Genesis File**: `https://raw.githubusercontent.com/EpixZone/EpixChain/main/artifacts/genesis/testnet/genesis.json`

## Hardware Requirements

### Minimum Requirements (Testnet)
- **CPU**: 4 cores
- **RAM**: 8 GB
- **Storage**: 500 GB SSD
- **Network**: 100 Mbps

### Recommended Requirements (Mainnet)
- **CPU**: 8+ cores
- **RAM**: 32 GB
- **Storage**: 2 TB NVMe SSD
- **Network**: 1 Gbps
- **Backup**: Regular automated backups

## Installation

### 1. Install Dependencies

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install required packages
sudo apt install -y curl wget jq git build-essential

# Install Go (version 1.21+)
wget https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
```

### 2. Build EpixChain

```bash
# Clone the repository
git clone https://github.com/EpixZone/EpixChain.git
cd EpixChain

# Build the binary
make install

# Verify installation
epixd version
```

## Testnet Setup

### 1. Initialize Node

```bash
# Set variables
MONIKER="your-validator-name"
CHAIN_ID="1917"
KEYRING="test"  # Use "os" for production

# Initialize node
epixd init $MONIKER --chain-id $CHAIN_ID --home ~/.epixd

# Create or import validator key
epixd keys add validator --keyring-backend $KEYRING --home ~/.epixd
# OR import existing key:
# epixd keys add validator --recover --keyring-backend $KEYRING --home ~/.epixd
```

### 2. Download Genesis File

```bash
# Download testnet genesis
curl -s https://raw.githubusercontent.com/EpixZone/EpixChain/main/artifacts/genesis/testnet/genesis.json > ~/.epixd/config/genesis.json

# Verify genesis file
epixd validate-genesis --home ~/.epixd
```

### 3. Configure Node

```bash
# Set persistent peers (update with current peers)
PEERS="peer1@ip1:26656,peer2@ip2:26656"
sed -i "s/persistent_peers = \"\"/persistent_peers = \"$PEERS\"/" ~/.epixd/config/config.toml

# Set minimum gas prices
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001aepix"/' ~/.epixd/config/app.toml

# Enable API and JSON-RPC (optional)
sed -i 's/enable = false/enable = true/' ~/.epixd/config/app.toml
sed -i 's/address = "127.0.0.1:8545"/address = "0.0.0.0:8545"/' ~/.epixd/config/app.toml
```

### 4. Start Node

```bash
# run as a service (recommended)
sudo tee /etc/systemd/system/epixd.service > /dev/null <<EOF
[Unit]
Description=EpixChain Node
After=network.target

[Service]
Type=simple
User=$USER
ExecStart=$(which epixd) start --home $HOME/.epixd
Restart=on-failure
RestartSec=3
LimitNOFILE=65535

[Install]
WantedBy=multi-user.target
EOF

sudo systemctl daemon-reload
sudo systemctl enable epixd
sudo systemctl start epixd


# Or start the node to run a quick test.
epixd start --home ~/.epixd
```

## Mainnet Setup

### 1. Initialize Node

```bash
# Set variables for mainnet
MONIKER="your-validator-name"
CHAIN_ID="1916"
KEYRING="os"  # Use secure keyring for mainnet

# Initialize node
epixd init $MONIKER --chain-id $CHAIN_ID --home ~/.epixd

# Create or import validator key (SECURE THIS KEY!)
epixd keys add validator --keyring-backend $KEYRING --home ~/.epixd
```

### 2. Download Genesis File

```bash
# Download mainnet genesis
curl -s https://raw.githubusercontent.com/EpixZone/EpixChain/main/artifacts/genesis/mainnet/genesis.json > ~/.epixd/config/genesis.json

# Verify genesis file
epixd validate-genesis --home ~/.epixd
```

### 3. Configure Node (Same as testnet but with mainnet peers)

```bash
# Set persistent peers (update with current mainnet peers)
PEERS="mainnet_peer1@ip1:26656,mainnet_peer2@ip2:26656"
sed -i "s/persistent_peers = \"\"/persistent_peers = \"$PEERS\"/" ~/.epixd/config/config.toml

# Set minimum gas prices
sed -i 's/minimum-gas-prices = ""/minimum-gas-prices = "0.001aepix"/' ~/.epixd/config/app.toml
```

## Validator Creation

### 1. Fund Your Validator Account

**Testnet:**
```bash
# Request testnet tokens from faucet or community
# Minimum required: 1 EPIX for self-delegation
```

**Mainnet:**
```bash
# Acquire EPIX tokens through exchanges or other means
# Minimum required: 1 EPIX for self-delegation
```

### 2. Create Validator Transaction

```bash
# Wait for node to sync completely
epixd status | jq .SyncInfo.catching_up

# Create validator (adjust values as needed)
epixd tx staking create-validator \
  --amount=1000000000000000000aepix \
  --pubkey=$(epixd tendermint show-validator --home ~/.epixd) \
  --moniker="$MONIKER" \
  --chain-id="$CHAIN_ID" \
  --commission-rate="0.05" \
  --commission-max-rate="0.20" \
  --commission-max-change-rate="0.01" \
  --min-self-delegation="1000000000000000000" \
  --gas="auto" \
  --gas-adjustment="1.2" \
  --gas-prices="0.001aepix" \
  --from=validator \
  --keyring-backend=$KEYRING \
  --home ~/.epixd
```

### 3. Verify Validator Creation

```bash
# Check validator status
epixd query staking validator $(epixd keys show validator --bech val -a --keyring-backend $KEYRING --home ~/.epixd)

# Check if validator is in active set
epixd query staking validators --limit 200 | grep -A 5 -B 5 $MONIKER
```

## Node Operations

### Essential Commands

```bash
# Check node status
epixd status

# Check sync status
epixd status | jq .SyncInfo

# Check validator info
epixd query staking validator $(epixd keys show validator --bech val -a --keyring-backend $KEYRING --home ~/.epixd)

# Check balance
epixd query bank balances $(epixd keys show validator -a --keyring-backend $KEYRING --home ~/.epixd)

# Delegate more tokens
epixd tx staking delegate $(epixd keys show validator --bech val -a --keyring-backend $KEYRING --home ~/.epixd) 1000000000000000000aepix --from validator --keyring-backend $KEYRING --home ~/.epixd

# Edit validator description
epixd tx staking edit-validator \
  --moniker="New Moniker" \
  --website="https://yourwebsite.com" \
  --identity="keybase_identity" \
  --details="Validator description" \
  --from=validator \
  --keyring-backend=$KEYRING \
  --home ~/.epixd
```

### Service Management

```bash
# Check service status
sudo systemctl status epixd

# View logs
sudo journalctl -u epixd -f

# Restart service
sudo systemctl restart epixd

# Stop service
sudo systemctl stop epixd
```

## Monitoring & Maintenance

### 1. Set Up Monitoring

```bash
# Install monitoring tools
# Prometheus, Grafana, or other monitoring solutions
# Monitor key metrics: uptime, missed blocks, disk space, memory usage
```

### 2. Regular Maintenance

- **Backup validator keys regularly**
- **Monitor disk space and clean up logs**
- **Keep the node software updated**
- **Monitor network upgrades and governance proposals**
- **Maintain high uptime (>95% recommended)**

### 3. Key Metrics to Monitor

- Node sync status
- Validator signing status
- Missed blocks count
- Disk space usage
- Memory and CPU usage
- Network connectivity

## Security Best Practices

### 1. Key Security
- **Never share your validator private key**
- **Use hardware security modules (HSM) for mainnet**
- **Backup keys in multiple secure locations**
- **Use strong passwords and 2FA**

### 2. Server Security
- **Use firewall to restrict access**
- **Keep system updated**
- **Use SSH keys instead of passwords**
- **Monitor for unauthorized access**
- **Use VPN for remote access**

### 3. Network Security
- **Use sentry nodes for DDoS protection**
- **Don't expose validator node directly to internet**
- **Use private networks when possible**

## Troubleshooting

### Common Issues

1. **Node not syncing**
   - Check peers configuration
   - Verify genesis file
   - Check network connectivity

2. **Validator not signing blocks**
   - Check if node is synced
   - Verify validator key
   - Check if validator is jailed

3. **High memory usage**
   - Adjust pruning settings
   - Increase system memory
   - Monitor for memory leaks

4. **Connection issues**
   - Check firewall settings
   - Verify peer connections
   - Check network configuration

### Getting Help

- **Discord**: Join the EpixChain community Discord
- **GitHub**: Report issues on the EpixChain repository
- **Documentation**: Check the official documentation
- **Community**: Engage with other validators

## Staking Parameters

- **Unbonding Period**: 21 days
- **Maximum Validators**: 100 (subject to governance)
- **Minimum Self Delegation**: 1 EPIX
- **Slashing Parameters**:
  - Double Sign: 5%
  - Downtime: 0.01%
  - Signed Blocks Window: 21,600 blocks
  - Minimum Signed Per Window: 5%

## Governance

Validators are expected to participate in governance by:
- Voting on proposals
- Engaging in community discussions
- Staying informed about network upgrades

## Support

For additional support and questions:
- **Community Discord**: [Join here]
- **GitHub Issues**: [Report bugs and issues]
- **Validator Chat**: [Validator-specific discussions]

---

**⚠️ Important**: Always test your setup on testnet before deploying to mainnet. Validator operations involve financial risk and require careful attention to security and operational practices.
