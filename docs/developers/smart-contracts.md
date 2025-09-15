# Smart Contract Development on EpixChain

EpixChain provides full Ethereum Virtual Machine (EVM) compatibility, allowing developers to deploy existing Ethereum smart contracts without modification while benefiting from lower fees and faster finality.

## Development Environment Setup

### Prerequisites

- **Node.js** 16+ and npm/yarn
- **Git** for version control
- **MetaMask** or other EVM wallet
- **Code Editor** (VS Code recommended)

### Recommended Tools

```bash
# Install Hardhat (recommended framework)
npm install --save-dev hardhat

# Install Foundry (alternative framework)
curl -L https://foundry.paradigm.xyz | bash
foundryup

# Install Truffle (legacy support)
npm install -g truffle

# Install Remix IDE dependencies
npm install -g @remix-project/remixd
```

## Network Configuration

### Hardhat Configuration

Create `hardhat.config.js`:

```javascript
require("@nomicfoundation/hardhat-toolbox");

module.exports = {
  solidity: {
    version: "0.8.19",
    settings: {
      optimizer: {
        enabled: true,
        runs: 200
      }
    }
  },
  networks: {
    epixchain: {
      url: "https://rpc.epixchain.com",
      chainId: 1916,
      accounts: [process.env.PRIVATE_KEY]
    },
    epixchain_testnet: {
      url: "http://localhost:8545",
      chainId: 1917,
      accounts: [process.env.PRIVATE_KEY]
    }
  },
  etherscan: {
    apiKey: {
      epixchain: "your-api-key"
    },
    customChains: [
      {
        network: "epixchain",
        chainId: 1916,
        urls: {
          apiURL: "https://api.scan.epix.zone/api",
          browserURL: "https://scan.epix.zone"
        }
      }
    ]
  }
};
```

### Foundry Configuration

Create `foundry.toml`:

```toml
[profile.default]
src = "src"
out = "out"
libs = ["lib"]
solc_version = "0.8.19"
optimizer = true
optimizer_runs = 200

[rpc_endpoints]
epixchain = "https://rpc.epixchain.com"
epixchain_testnet = "http://localhost:8545"

[etherscan]
epixchain = { key = "${ETHERSCAN_API_KEY}", url = "https://api.scan.epix.zone/api" }
```

## Smart Contract Examples

### Basic ERC20 Token

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract EpixToken is ERC20, Ownable {
    constructor(
        string memory name,
        string memory symbol,
        uint256 initialSupply
    ) ERC20(name, symbol) {
        _mint(msg.sender, initialSupply * 10**decimals());
    }

    function mint(address to, uint256 amount) public onlyOwner {
        _mint(to, amount);
    }

    function burn(uint256 amount) public {
        _burn(msg.sender, amount);
    }
}
```

### Cross-Chain Bridge Contract

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "./interfaces/IIBCPrecompile.sol";

contract CrossChainBridge {
    IIBCPrecompile constant IBC = IIBCPrecompile(0x0000000000000000000000000000000000000065);
    
    event TokensBridged(
        address indexed sender,
        string destinationChain,
        string recipient,
        uint256 amount
    );

    function bridgeTokens(
        string memory destinationChain,
        string memory recipient,
        uint256 amount
    ) external payable {
        require(msg.value >= amount, "Insufficient value");
        
        // Use IBC precompile for cross-chain transfer
        IBC.transfer(
            "transfer",
            "channel-0",
            destinationChain,
            recipient,
            amount,
            block.timestamp + 3600 // 1 hour timeout
        );
        
        emit TokensBridged(msg.sender, destinationChain, recipient, amount);
    }
}
```

### Governance Integration Contract

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "./interfaces/IGovernancePrecompile.sol";

contract DAOProposal {
    IGovernancePrecompile constant GOV = IGovernancePrecompile(0x0000000000000000000000000000000000000067);
    
    struct Proposal {
        uint256 id;
        string title;
        string description;
        address proposer;
        uint256 votingStart;
        uint256 votingEnd;
        bool executed;
    }
    
    mapping(uint256 => Proposal) public proposals;
    uint256 public proposalCount;
    
    function submitProposal(
        string memory title,
        string memory description,
        bytes memory content
    ) external returns (uint256) {
        // Submit proposal through governance precompile
        uint256 proposalId = GOV.submitProposal(content, title, description);
        
        proposals[proposalCount] = Proposal({
            id: proposalId,
            title: title,
            description: description,
            proposer: msg.sender,
            votingStart: block.timestamp,
            votingEnd: block.timestamp + 7 days,
            executed: false
        });
        
        return proposalCount++;
    }
    
    function vote(uint256 proposalId, bool support) external {
        GOV.vote(proposalId, support ? 1 : 0, "");
    }
}
```

## Deployment Guide

### Using Hardhat

```bash
# Compile contracts
npx hardhat compile

# Deploy to testnet
npx hardhat run scripts/deploy.js --network epixchain_testnet

# Deploy to mainnet
npx hardhat run scripts/deploy.js --network epixchain

# Verify contract
npx hardhat verify --network epixchain CONTRACT_ADDRESS "Constructor" "Arguments"
```

### Using Foundry

```bash
# Build contracts
forge build

# Deploy to testnet
forge create --rpc-url epixchain_testnet \
  --private-key $PRIVATE_KEY \
  src/MyContract.sol:MyContract \
  --constructor-args "arg1" "arg2"

# Deploy to mainnet
forge create --rpc-url epixchain \
  --private-key $PRIVATE_KEY \
  src/MyContract.sol:MyContract \
  --constructor-args "arg1" "arg2"

# Verify contract
forge verify-contract \
  --chain-id 1916 \
  --num-of-optimizations 200 \
  --compiler-version v0.8.19 \
  CONTRACT_ADDRESS \
  src/MyContract.sol:MyContract \
  --etherscan-api-key $ETHERSCAN_API_KEY
```

## Precompiled Contracts

EpixChain provides several precompiled contracts for enhanced functionality:

### Bank Precompile (0x64)

```solidity
interface IBankPrecompile {
    function balanceOf(address account, string memory denom) external view returns (uint256);
    function totalSupply(string memory denom) external view returns (uint256);
    function transfer(address to, uint256 amount, string memory denom) external returns (bool);
}
```

### Staking Precompile (0x65)

```solidity
interface IStakingPrecompile {
    function delegate(string memory validator, uint256 amount) external returns (bool);
    function undelegate(string memory validator, uint256 amount) external returns (bool);
    function redelegate(string memory srcValidator, string memory dstValidator, uint256 amount) external returns (bool);
    function getDelegation(address delegator, string memory validator) external view returns (uint256);
}
```

### Governance Precompile (0x67)

```solidity
interface IGovernancePrecompile {
    function submitProposal(bytes memory content, string memory title, string memory description) external returns (uint256);
    function vote(uint256 proposalId, uint256 option, string memory metadata) external returns (bool);
    function getProposal(uint256 proposalId) external view returns (bytes memory);
}
```

## Gas Optimization

### EpixChain-Specific Optimizations

1. **Use Precompiles**: Leverage native functionality through precompiled contracts
2. **Batch Operations**: Combine multiple operations in single transactions
3. **Efficient Storage**: Minimize storage operations and use events for data
4. **Native Tokens**: Use native EPIX instead of wrapped tokens when possible

### Gas Price Strategies

```javascript
// Dynamic gas pricing
const gasPrice = await web3.eth.getGasPrice();
const optimizedGasPrice = Math.floor(gasPrice * 1.1); // 10% buffer

// Transaction with optimized gas
const tx = {
  from: account,
  to: contractAddress,
  data: contractData,
  gas: estimatedGas,
  gasPrice: optimizedGasPrice
};
```

## Testing Framework

### Hardhat Testing

```javascript
const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("EpixToken", function () {
  let token;
  let owner;
  let addr1;

  beforeEach(async function () {
    [owner, addr1] = await ethers.getSigners();
    
    const EpixToken = await ethers.getContractFactory("EpixToken");
    token = await EpixToken.deploy("Test Token", "TEST", 1000000);
    await token.deployed();
  });

  it("Should mint tokens correctly", async function () {
    await token.mint(addr1.address, 1000);
    expect(await token.balanceOf(addr1.address)).to.equal(1000);
  });

  it("Should handle cross-chain operations", async function () {
    // Test IBC precompile integration
    const bridge = await ethers.getContractAt("CrossChainBridge", bridgeAddress);
    await bridge.bridgeTokens("cosmos-hub", "cosmos1...", 1000, { value: 1000 });
  });
});
```

### Foundry Testing

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "forge-std/Test.sol";
import "../src/EpixToken.sol";

contract EpixTokenTest is Test {
    EpixToken token;
    address owner = address(0x1);
    address user = address(0x2);

    function setUp() public {
        vm.prank(owner);
        token = new EpixToken("Test Token", "TEST", 1000000);
    }

    function testMint() public {
        vm.prank(owner);
        token.mint(user, 1000);
        assertEq(token.balanceOf(user), 1000);
    }

    function testBurn() public {
        vm.prank(owner);
        token.mint(user, 1000);
        
        vm.prank(user);
        token.burn(500);
        assertEq(token.balanceOf(user), 500);
    }
}
```

## Best Practices

### Security Considerations

1. **Use Latest Solidity**: Always use the latest stable version
2. **Audit Contracts**: Get professional audits for mainnet deployments
3. **Test Thoroughly**: Comprehensive testing on testnet before mainnet
4. **Access Controls**: Implement proper permission systems
5. **Reentrancy Protection**: Use OpenZeppelin's ReentrancyGuard

### Performance Optimization

1. **Minimize Storage**: Use events for non-critical data
2. **Batch Operations**: Combine multiple calls when possible
3. **Efficient Algorithms**: Optimize loops and calculations
4. **Gas Estimation**: Always estimate gas before transactions

### Integration Patterns

1. **Precompile Usage**: Leverage native blockchain functionality
2. **Cross-Chain Design**: Plan for multi-chain interactions
3. **Governance Integration**: Enable community control where appropriate
4. **Upgrade Patterns**: Use proxy patterns for upgradeable contracts

## Resources and Tools

### Development Tools

- **Hardhat**: [hardhat.org](https://hardhat.org)
- **Foundry**: [getfoundry.sh](https://getfoundry.sh)
- **Remix IDE**: [remix.ethereum.org](https://remix.ethereum.org)
- **OpenZeppelin**: [openzeppelin.com/contracts](https://openzeppelin.com/contracts)

### EpixChain Resources

- **RPC Endpoint**: `https://rpc.epixchain.com`
- **Block Explorer**: [scan.epix.zone](https://scan.epix.zone)
- **Faucet**: [faucet.epix.zone](https://faucet.epix.zone) (testnet)
- **GitHub**: [github.com/EpixZone/EpixChain](https://github.com/EpixZone/EpixChain)

### Community Support

- **Discord**: [discord.gg/epix](https://discord.gg/epix)
- **Developer Chat**: #developers channel
- **GitHub Issues**: Bug reports and feature requests
- **Documentation**: Comprehensive guides and references

---

*Start building the future of decentralized applications on EpixChain today!*
