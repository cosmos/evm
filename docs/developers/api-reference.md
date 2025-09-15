# API Reference

EpixChain provides multiple API interfaces for different use cases. This comprehensive reference covers all available endpoints and their usage.

## API Endpoints

### Mainnet
- **JSON-RPC**: `https://rpc.epixchain.com`
- **REST API**: `https://api.epixchain.com`
- **gRPC**: `grpc.epixchain.com:9090`
- **WebSocket**: `wss://ws.epixchain.com`

### Devnet
- **JSON-RPC**: `http://localhost:8545`
- **REST API**: `http://localhost:1317`
- **gRPC**: `localhost:9090`
- **WebSocket**: `ws://localhost:8546`

## JSON-RPC API (Ethereum Compatible)

EpixChain supports all standard Ethereum JSON-RPC methods for seamless integration with existing tools.

### Connection Examples

```javascript
// Web3.js
const Web3 = require('web3');
const web3 = new Web3('https://rpc.epixchain.com');

// Ethers.js
const { ethers } = require('ethers');
const provider = new ethers.providers.JsonRpcProvider('https://rpc.epixchain.com');

// Axios (raw HTTP)
const axios = require('axios');
const rpc = axios.create({
  baseURL: 'https://rpc.epixchain.com',
  headers: { 'Content-Type': 'application/json' }
});
```

### Core Methods

#### eth_getBalance
Get account balance in wei.

```javascript
// Request
{
  "jsonrpc": "2.0",
  "method": "eth_getBalance",
  "params": ["0x742d35Cc6634C0532925a3b8D4C9db96590c6C87", "latest"],
  "id": 1
}

// Response
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": "0x1b1ae4d6e2ef500000"
}

// Usage
const balance = await web3.eth.getBalance('0x742d35Cc6634C0532925a3b8D4C9db96590c6C87');
console.log(web3.utils.fromWei(balance, 'ether'), 'EPIX');
```

#### eth_sendTransaction
Send a transaction.

```javascript
// Request
{
  "jsonrpc": "2.0",
  "method": "eth_sendTransaction",
  "params": [{
    "from": "0x742d35Cc6634C0532925a3b8D4C9db96590c6C87",
    "to": "0x8ba1f109551bD432803012645Hac136c",
    "value": "0xde0b6b3a7640000",
    "gas": "0x5208",
    "gasPrice": "0x9184e72a000"
  }],
  "id": 1
}

// Usage
const txHash = await web3.eth.sendTransaction({
  from: account,
  to: '0x8ba1f109551bD432803012645Hac136c',
  value: web3.utils.toWei('1', 'ether'),
  gas: 21000
});
```

#### eth_call
Execute a contract call without creating a transaction.

```javascript
// Contract call
const result = await contract.methods.balanceOf(address).call();

// Raw call
const data = contract.methods.balanceOf(address).encodeABI();
const result = await web3.eth.call({
  to: contractAddress,
  data: data
});
```

### Block and Transaction Methods

```javascript
// Get latest block
const block = await web3.eth.getBlock('latest');

// Get transaction by hash
const tx = await web3.eth.getTransaction('0x...');

// Get transaction receipt
const receipt = await web3.eth.getTransactionReceipt('0x...');

// Get logs
const logs = await web3.eth.getPastLogs({
  fromBlock: 'latest',
  toBlock: 'latest',
  address: contractAddress
});
```

## REST API (Cosmos SDK)

The REST API provides access to Cosmos SDK functionality and blockchain state.

### Base URL Structure
```
https://api.epixchain.com/cosmos/{module}/{version}/{endpoint}
```

### Authentication
Most endpoints are public. For transaction submission, use signed transactions.

### Core Endpoints

#### Bank Module

**Get Balance**
```bash
GET /cosmos/bank/v1beta1/balances/{address}

# Example
curl https://api.epixchain.com/cosmos/bank/v1beta1/balances/epix1...
```

**Get Supply**
```bash
GET /cosmos/bank/v1beta1/supply

# Response
{
  "supply": [
    {
      "denom": "aepix",
      "amount": "42000000000000000000000000000"
    }
  ]
}
```

#### Staking Module

**Get Validators**
```bash
GET /cosmos/staking/v1beta1/validators

# With pagination
GET /cosmos/staking/v1beta1/validators?pagination.limit=10&pagination.offset=0
```

**Get Delegations**
```bash
GET /cosmos/staking/v1beta1/delegations/{delegator_addr}

# Example response
{
  "delegation_responses": [
    {
      "delegation": {
        "delegator_address": "epix1...",
        "validator_address": "epixvaloper1...",
        "shares": "1000000000000000000000.000000000000000000"
      },
      "balance": {
        "denom": "aepix",
        "amount": "1000000000000000000000"
      }
    }
  ]
}
```

#### Governance Module

**Get Proposals**
```bash
GET /cosmos/gov/v1beta1/proposals

# Get specific proposal
GET /cosmos/gov/v1beta1/proposals/{proposal_id}
```

**Get Votes**
```bash
GET /cosmos/gov/v1beta1/proposals/{proposal_id}/votes
```

#### EpixMint Module

**Get Parameters**
```bash
GET /epixmint/v1/params

# Response
{
  "params": {
    "mint_denom": "aepix",
    "blocks_per_year": "5256000",
    "max_supply": "42000000000000000000000000000",
    "annual_reduction_factor": "0.750000000000000000"
  }
}
```

**Get Annual Provisions**
```bash
GET /epixmint/v1/annual_provisions

# Response
{
  "annual_provisions": "10527000000000000000000000000"
}
```

### Transaction Submission

**Broadcast Transaction**
```bash
POST /cosmos/tx/v1beta1/txs

# Request body
{
  "tx_bytes": "base64_encoded_transaction",
  "mode": "BROADCAST_MODE_SYNC"
}
```

## gRPC API

High-performance API for advanced integrations.

### Connection

```javascript
// Node.js gRPC client
const grpc = require('@grpc/grpc-js');
const protoLoader = require('@grpc/proto-loader');

// Load proto definitions
const packageDefinition = protoLoader.loadSync('cosmos/bank/v1beta1/query.proto');
const bankProto = grpc.loadPackageDefinition(packageDefinition);

// Create client
const client = new bankProto.cosmos.bank.v1beta1.Query(
  'grpc.epixchain.com:9090',
  grpc.credentials.createInsecure()
);

// Query balance
client.Balance({
  address: 'epix1...',
  denom: 'aepix'
}, (error, response) => {
  console.log(response.balance);
});
```

### Available Services

- `cosmos.bank.v1beta1.Query`
- `cosmos.staking.v1beta1.Query`
- `cosmos.gov.v1beta1.Query`
- `cosmos.distribution.v1beta1.Query`
- `epixmint.v1.Query`
- `ethermint.evm.v1.Query`

## WebSocket API

Real-time event streaming for live applications.

### Connection

```javascript
const WebSocket = require('ws');
const ws = new WebSocket('wss://ws.epixchain.com');

ws.on('open', () => {
  // Subscribe to new blocks
  ws.send(JSON.stringify({
    jsonrpc: '2.0',
    method: 'eth_subscribe',
    params: ['newHeads'],
    id: 1
  }));
});

ws.on('message', (data) => {
  const response = JSON.parse(data);
  console.log('New block:', response.params.result);
});
```

### Subscription Types

**New Blocks**
```javascript
ws.send(JSON.stringify({
  jsonrpc: '2.0',
  method: 'eth_subscribe',
  params: ['newHeads'],
  id: 1
}));
```

**Pending Transactions**
```javascript
ws.send(JSON.stringify({
  jsonrpc: '2.0',
  method: 'eth_subscribe',
  params: ['newPendingTransactions'],
  id: 2
}));
```

**Contract Logs**
```javascript
ws.send(JSON.stringify({
  jsonrpc: '2.0',
  method: 'eth_subscribe',
  params: ['logs', {
    address: '0x...',
    topics: ['0x...']
  }],
  id: 3
}));
```

## Rate Limits and Best Practices

### Rate Limits

| Endpoint Type | Limit | Window |
|---------------|-------|--------|
| JSON-RPC | 100 req/sec | Per IP |
| REST API | 50 req/sec | Per IP |
| gRPC | 200 req/sec | Per connection |
| WebSocket | 10 subscriptions | Per connection |

### Best Practices

1. **Use Appropriate API**: Choose the right API for your use case
2. **Implement Caching**: Cache responses when possible
3. **Handle Errors**: Implement proper error handling and retries
4. **Use Pagination**: For large datasets, use pagination parameters
5. **WebSocket Management**: Properly handle WebSocket reconnections

### Error Handling

```javascript
// JSON-RPC error handling
try {
  const result = await web3.eth.getBalance(address);
} catch (error) {
  if (error.code === -32602) {
    console.log('Invalid parameters');
  } else if (error.code === -32603) {
    console.log('Internal error');
  }
}

// REST API error handling
try {
  const response = await axios.get('/cosmos/bank/v1beta1/balances/invalid');
} catch (error) {
  if (error.response.status === 400) {
    console.log('Bad request:', error.response.data.message);
  }
}
```

## SDK Integration

### CosmJS (Cosmos SDK)

```javascript
const { SigningStargateClient } = require('@cosmjs/stargate');
const { DirectSecp256k1HdWallet } = require('@cosmjs/proto-signing');

// Create wallet
const wallet = await DirectSecp256k1HdWallet.fromMnemonic(mnemonic, {
  prefix: 'epix'
});

// Connect to chain
const client = await SigningStargateClient.connectWithSigner(
  'https://rpc.epixchain.com',
  wallet
);

// Send transaction
const result = await client.sendTokens(
  senderAddress,
  recipientAddress,
  [{ denom: 'aepix', amount: '1000000000000000000' }],
  'auto'
);
```

### Ethers.js (EVM)

```javascript
const { ethers } = require('ethers');

// Connect to provider
const provider = new ethers.providers.JsonRpcProvider('https://rpc.epixchain.com');

// Create wallet
const wallet = new ethers.Wallet(privateKey, provider);

// Send transaction
const tx = await wallet.sendTransaction({
  to: recipientAddress,
  value: ethers.utils.parseEther('1.0')
});
```

## Testing and Development

### Local Development

```bash
# Start local node
epixd start --rpc.laddr tcp://0.0.0.0:26657

# Enable CORS for development
epixd start --rpc.cors_allowed_origins="*"
```

### API Testing Tools

- **Postman**: REST API testing
- **Insomnia**: GraphQL and REST testing
- **curl**: Command-line testing
- **grpcurl**: gRPC testing

---

*This API reference provides comprehensive access to EpixChain's functionality. For additional support, join our [Discord community](https://discord.gg/epix).*
