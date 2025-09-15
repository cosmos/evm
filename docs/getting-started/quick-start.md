# Quick Start Guide

Get up and running with EpixChain in just a few minutes! This guide will help you connect to the network, set up your wallet, and make your first transaction.

## Prerequisites

Before you begin, ensure you have:
- A modern web browser (Chrome, Firefox, Safari, or Edge)
- MetaMask or another EVM-compatible wallet installed
- Basic understanding of blockchain and cryptocurrency concepts

## Step 1: Add EpixChain to Your Wallet

### MetaMask Configuration

1. **Open MetaMask** and click on the network dropdown (usually shows "Ethereum Mainnet")

2. **Add Custom Network** with the following details:

#### Mainnet Configuration
```
Network Name: EpixChain Mainnet
RPC URL: https://rpc.epixchain.com
Chain ID: 1916
Currency Symbol: EPIX
Block Explorer: https://scan.epix.zone
```

#### Testnet Configuration (for development)
```
Network Name: EpixChain Testnet
RPC URL: http://localhost:8545
Chain ID: 1917
Currency Symbol: EPIX
Block Explorer: http://localhost:8080
```

3. **Save** the network configuration

4. **Switch** to the EpixChain network

### Alternative Wallets

EpixChain is compatible with any EVM wallet:
- **Trust Wallet**: Use the same network configuration
- **WalletConnect**: Connect through supported dApps
- **Coinbase Wallet**: Add as custom network
- **Brave Wallet**: Built-in support for custom networks

## Step 2: Get EPIX Tokens

### For Mainnet

1. **Claim Airdrop** (if eligible):
   - Visit [epix.zone](https://epix.zone)
   - Connect your wallet
   - Check eligibility and claim tokens

2. **Purchase EPIX**:
   - Use supported exchanges (coming soon)
   - Cross-chain bridges from other networks
   - DEX trading on EpixChain

3. **Earn EPIX**:
   - Stake tokens with validators
   - Participate in governance
   - Provide liquidity to DEXs

### For Testnet

1. **Faucet** (development only):
   ```bash
   # Request testnet tokens
   curl -X POST https://faucet.epix.zone/request \
     -H "Content-Type: application/json" \
     -d '{"address": "YOUR_WALLET_ADDRESS"}'
   ```

2. **Local Development**:
   ```bash
   # If running local node
   epixd tx bank send validator YOUR_ADDRESS 1000000000000000000aepix \
     --chain-id epix_1917-1 \
     --keyring-backend test
   ```

## Step 3: Make Your First Transaction

### Simple Transfer

1. **Open your wallet** and ensure you're on the EpixChain network

2. **Send EPIX** to another address:
   - Click "Send" in your wallet
   - Enter recipient address
   - Enter amount (remember: 1 EPIX = 10^18 aepix)
   - Confirm transaction

3. **Check transaction** on the block explorer

### Interact with Smart Contracts

1. **Visit a dApp** built on EpixChain
2. **Connect your wallet** when prompted
3. **Approve transactions** as needed
4. **Monitor gas fees** (should be very low!)

## Step 4: Explore the Ecosystem

### Block Explorer

Visit [scan.epix.zone](https://scan.epix.zone) to:
- View transaction history
- Explore blocks and validators
- Check contract interactions
- Monitor network statistics

### Governance Participation

1. **Stake your EPIX** with a validator
2. **View active proposals** on governance platforms
3. **Vote on proposals** that interest you
4. **Submit proposals** for network improvements

### Cross-Chain Activities

1. **IBC Transfers**: Move assets to/from other Cosmos chains
2. **Bridge Assets**: Use bridges to connect with Ethereum and other networks
3. **Multi-Chain DeFi**: Participate in cross-chain protocols

## Common Commands

### Using epixd CLI

```bash
# Check balance
epixd query bank balances YOUR_ADDRESS

# Send tokens
epixd tx bank send YOUR_ADDRESS RECIPIENT_ADDRESS 1000000000000000000aepix \
  --chain-id epix_1916-1 \
  --gas auto \
  --gas-adjustment 1.5

# Delegate to validator
epixd tx staking delegate VALIDATOR_ADDRESS 1000000000000000000aepix \
  --from YOUR_KEY \
  --chain-id epix_1916-1

# Vote on proposal
epixd tx gov vote 1 yes \
  --from YOUR_KEY \
  --chain-id epix_1916-1
```

### Using Web3 Libraries

```javascript
// Connect to EpixChain
const Web3 = require('web3');
const web3 = new Web3('https://rpc.epixchain.com');

// Check balance
const balance = await web3.eth.getBalance('YOUR_ADDRESS');
console.log('Balance:', web3.utils.fromWei(balance, 'ether'), 'EPIX');

// Send transaction
const tx = {
  from: 'YOUR_ADDRESS',
  to: 'RECIPIENT_ADDRESS',
  value: web3.utils.toWei('1', 'ether'),
  gas: 21000
};

const signedTx = await web3.eth.accounts.signTransaction(tx, 'YOUR_PRIVATE_KEY');
const receipt = await web3.eth.sendSignedTransaction(signedTx.rawTransaction);
```

## Troubleshooting

### Common Issues

**"Transaction Failed"**
- Check you have enough EPIX for gas fees
- Verify you're on the correct network
- Ensure recipient address is valid

**"Network Not Found"**
- Double-check RPC URL and Chain ID
- Try switching networks and back
- Clear browser cache if needed

**"Insufficient Funds"**
- Verify your EPIX balance
- Account for gas fees in transactions
- Check if tokens are staked/locked

### Getting Help

- **Discord**: [Join our community](https://discord.gg/epix) for real-time support
- **Documentation**: Browse these docs for detailed guides
- **GitHub**: [Report issues](https://github.com/EpixZone/EpixChain/issues) or contribute
- **Twitter**: [@zone_epix](https://x.com/zone_epix) for updates

## Next Steps

Now that you're connected to EpixChain:

1. **[Set up staking](../users/staking.md)** to earn rewards
2. **[Explore governance](../users/governance.md)** to participate in decisions
3. **[Try cross-chain transfers](../users/ibc-transfers.md)** to other networks
4. **[Build applications](../developers/smart-contracts.md)** on EpixChain
5. **[Run a validator](../validators/setup.md)** to secure the network

Welcome to the EpixChain ecosystem! 🚀
