# EpixChain Governance Guide

This guide explains how to create and submit governance proposals to modify EpixChain parameters, specifically focusing on the EpixMint module parameters.

## Table of Contents

- [Overview](#overview)
- [EpixMint Parameters](#epixmint-parameters)
- [Creating a Governance Proposal](#creating-a-governance-proposal)
- [Submitting the Proposal](#submitting-the-proposal)
- [Funding the Proposal](#funding-the-proposal)
- [Voting Process](#voting-process)
- [Example: Changing Block Time](#example-changing-block-time)
- [Troubleshooting](#troubleshooting)

## Overview

EpixChain uses on-chain governance to allow stakeholders to propose and vote on parameter changes. The governance process ensures that important network changes are decided democratically by token holders and validators.

### Governance Process Flow

1. **Proposal Creation** - Create a JSON proposal file
2. **Proposal Submission** - Submit the proposal on-chain
3. **Deposit Period** - Fund the proposal to move it to voting
4. **Voting Period** - Validators and delegators vote
5. **Execution** - If passed, changes are automatically applied

## EpixMint Parameters

The EpixMint module controls token emission with the following parameters:

| Parameter | Type | Description | Current Default |
|-----------|------|-------------|-----------------|
| `mint_denom` | string | Token denomination | "aepix" |
| `initial_annual_mint_amount` | string | Starting emission amount | 10.527B EPIX |
| `annual_reduction_rate` | string | Annual reduction percentage | 0.25 (25%) |
| `block_time_seconds` | uint64 | Expected block time | 6 seconds |
| `max_supply` | string | Maximum total supply | 42B EPIX |
| `community_pool_rate` | string | Community pool allocation | 0.02 (2%) |
| `staking_rewards_rate` | string | Staking rewards allocation | 0.98 (98%) |

### Parameter Format Notes

- Amounts are in extended precision (multiply by 10^18)
- Rates are decimal strings (e.g., "0.25" for 25%)
- All parameters must be included in proposals

### How EpixMint Automatically Adjusts

The EpixMint module is **"block-time aware"** - it automatically adjusts token emission when block times change. Here's how it works in simple terms:

#### 1. **Dynamic Block Time Detection**
- The system monitors actual block production times (not just the configured parameter)
- It calculates the average time between the last 100 blocks
- For new chains (under 100 blocks), it uses the configured `block_time_seconds` parameter

#### 2. **Automatic Emission Adjustment**
When you change the `block_time_seconds` parameter through governance:

**Example: Changing from 6 seconds to 2 seconds**
- **Before**: 6-second blocks = 5,256,000 blocks per year
- **After**: 2-second blocks = 15,768,000 blocks per year (3x more blocks)
- **Result**: Each block mints 3x fewer tokens to maintain the same annual emission

#### 3. **The Math Behind It**
```
Annual Emission = 10.527B EPIX (year 1)
Blocks Per Year = 31,536,000 seconds ÷ block_time_seconds
Tokens Per Block = Annual Emission ÷ Blocks Per Year

6-second blocks: 10.527B ÷ 5,256,000 = ~2,002 EPIX per block
2-second blocks: 10.527B ÷ 15,768,000 = ~667 EPIX per block
```

#### 4. **Why This Matters**
- **Consistent Economics**: Annual emission stays the same regardless of block time
- **No Manual Intervention**: Changes happen automatically when governance updates block time
- **Smooth Transition**: No disruption to the tokenomics schedule
- **Future-Proof**: Works with any block time (1-60 seconds)

## Creating a Governance Proposal

### 1. Proposal Structure

Create a JSON file with the following structure:

```json
{
  "messages": [
    {
      "@type": "/epixmint.v1.MsgUpdateParams",
      "authority": "epix10d07y265gmmuvt4z0w9aw880jnsr700j0fas3g",
      "params": {
        "mint_denom": "aepix",
        "initial_annual_mint_amount": "10527000000000000000000000000",
        "annual_reduction_rate": "0.250000000000000000",
        "block_time_seconds": 6,
        "max_supply": "42000000000000000000000000000",
        "community_pool_rate": "0.020000000000000000",
        "staking_rewards_rate": "0.980000000000000000"
      }
    }
  ],
  "metadata": "Update EpixMint Parameters",
  "deposit": "10000000000000000000000aepix",
  "title": "Your Proposal Title",
  "summary": "Detailed description of the changes and rationale"
}
```

### 2. Key Components

- **@type**: Must be `/epixmint.v1.MsgUpdateParams`
- **authority**: Governance module address (fixed)
- **params**: All EpixMint parameters (all required)
- **deposit**: Minimum 10,000 EPIX in extended precision
- **title**: Clear, descriptive title
- **summary**: Detailed explanation of changes

## Submitting the Proposal

### 1. Prerequisites

- Ensure you have a funded account
- Know your chain ID
- Have the proposal JSON file ready

### 2. Get Chain Information

```bash
# Check chain ID
epixd status | jq -r '.NodeInfo.network'

# Check your account balance
epixd query bank balances $(epixd keys show validator -a --keyring-backend test)

# List available keys
epixd keys list --keyring-backend test
```

### 3. Submit Command

```bash
epixd tx gov submit-proposal docs/your-proposal.json \
  --from <key-name> \
  --chain-id <chain-id> \
  --gas auto \
  --gas-adjustment 1.5 \
  --fees 1000000000000000000aepix \
  --keyring-backend test
```

**Example:**
```bash
epixd tx gov submit-proposal docs/epixmint-governance-proposal-example.json \
  --from validator \
  --chain-id epix_1916-1 \
  --gas auto \
  --gas-adjustment 1.5 \
  --fees 1000000000000000000aepix \
  --keyring-backend test
```

## Funding the Proposal

After submission, proposals need funding to enter the voting period.

### 1. Find Your Proposal

```bash
# List all proposals
epixd query gov proposals --chain-id <chain-id>

# Check specific proposal
epixd query gov proposal <proposal-id> --chain-id <chain-id>
```

### 2. Fund the Proposal

```bash
epixd tx gov deposit <proposal-id> 10000000000000000000000aepix \
  --from <key-name> \
  --chain-id <chain-id> \
  --gas auto \
  --gas-adjustment 1.5 \
  --fees 1000000000000000000aepix \
  --keyring-backend test
```

## Voting Process

Once funded, the proposal enters the voting period.

### 1. Vote on Proposal

```bash
epixd tx gov vote <proposal-id> <vote-option> \
  --from <key-name> \
  --chain-id <chain-id> \
  --gas auto \
  --gas-adjustment 1.5 \
  --fees 1000000000000000000aepix \
  --keyring-backend test
```

**Vote Options:**
- `yes` - Support the proposal
- `no` - Oppose the proposal
- `abstain` - Abstain from voting
- `no_with_veto` - Oppose with veto (burns deposit if >33.4%)

### 2. Check Voting Status

```bash
# Check proposal status
epixd query gov proposal <proposal-id> --chain-id <chain-id>

# Check tally
epixd query gov tally <proposal-id> --chain-id <chain-id>
```

## Example: Changing Block Time

Here's a complete example of changing the block time from 6 seconds to 2 seconds:

### 1. Create Proposal File

Save as `docs/block-time-proposal.json`:

```json
{
  "messages": [
    {
      "@type": "/epixmint.v1.MsgUpdateParams",
      "authority": "epix10d07y265gmmuvt4z0w9aw880jnsr700j0fas3g",
      "params": {
        "mint_denom": "aepix",
        "initial_annual_mint_amount": "10527000000000000000000000000",
        "annual_reduction_rate": "0.250000000000000000",
        "block_time_seconds": 2,
        "max_supply": "42000000000000000000000000000",
        "community_pool_rate": "0.020000000000000000",
        "staking_rewards_rate": "0.980000000000000000"
      }
    }
  ],
  "metadata": "Update Block Time to 2 Seconds",
  "deposit": "10000000000000000000000aepix",
  "title": "Update Block Time to 2 Seconds",
  "summary": "Reduce block time from 6 seconds to 2 seconds to improve transaction throughput and user experience."
}
```

### 2. Submit and Fund

```bash
# Submit
epixd tx gov submit-proposal docs/block-time-proposal.json \
  --from YOUR_WALLET \
  --chain-id epix_1916-1 \
  --gas auto \
  --gas-adjustment 1.5 \
  --fees 1000000000000000000aepix \
  --keyring-backend YOUR_KEY

# Fund (replace 1 with actual proposal ID)
epixd tx gov deposit 1 10000000000000000000000aepix \
  --from YOUR_WALLET \
  --chain-id epix_1916-1 \
  --gas auto \
  --gas-adjustment 1.5 \
  --fees 1000000000000000000aepix \
  --keyring-backend YOUR_KEY

# Vote
epixd tx gov vote 1 yes \
  --from YOUR_WALLET \
  --chain-id epix_1916-1 \
  --gas auto \
  --gas-adjustment 1.5 \
  --fees 1000000000000000000aepix \
  --keyring-backend YOUR_KEY
```

## Troubleshooting

### Common Errors

1. **Chain ID Mismatch**
   ```
   Error: signature verification failed; please verify account number and chain-id
   ```
   **Solution:** Use correct chain ID from `epixd status | jq -r '.NodeInfo.network'`

2. **Insufficient Funds**
   ```
   Error: insufficient funds
   ```
   **Solution:** Ensure account has enough balance for deposit + fees

3. **Invalid Parameters**
   ```
   Error: invalid parameter value
   ```
   **Solution:** Check parameter formats and ensure all required fields are included

### Getting Help

- Check proposal status: `epixd query gov proposal <id>`
- View transaction details: `epixd query tx <hash>`
- Check account info: `epixd query auth account <address>`

## Best Practices

1. **Test First**: Test proposals on testnets before mainnet
2. **Clear Communication**: Provide detailed rationale in proposal summary
3. **Community Engagement**: Discuss proposals with community before submission
4. **Parameter Validation**: Double-check all parameter values and formats
5. **Timing**: Consider voting periods and community availability

---

For more information about EpixChain governance, see the [Cosmos SDK Governance Documentation](https://docs.cosmos.network/main/modules/gov).
