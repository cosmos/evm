# EpixMint Module

The EpixMint module implements a dynamic token emission system with exponential decay for the EpixChain blockchain.

## Overview

The EpixMint module replaces the standard Cosmos SDK mint module with a custom implementation that provides:

- **Dynamic emission rates** with smooth exponential decay
- **Block-time awareness** for consistent emission regardless of consensus changes
- **Maximum supply protection** to prevent exceeding the 42B EPIX cap
- **Governance-controlled parameters** for flexibility

## Tokenomics

### Emission Schedule

- **Initial Annual Emission**: 10.527 billion EPIX in year 1
- **Annual Reduction Rate**: 25% per year
- **Total Duration**: Approximately 20 years to reach max supply
- **Maximum Supply**: 42 billion EPIX (hard cap with protection)

#### Emission Curve Visualization

```mermaid
%%{init: {'theme':'dark'}}%%
xychart-beta
    title "EpixChain Annual Emission Schedule (25% Reduction) - Years 1-20"
    x-axis [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20]
    y-axis "Annual Emission (Billion EPIX)" 0 --> 12
    line [10.527, 7.895, 5.921, 4.441, 3.331, 2.498, 1.874, 1.405, 1.054, 0.790, 0.593, 0.445, 0.334, 0.250, 0.188, 0.141, 0.106, 0.079, 0.059, 0.045]
```

#### Cumulative Supply Growth

```mermaid
%%{init: {'theme':'dark'}}%%
xychart-beta
    title "EpixChain Cumulative Supply Growth - Years 1-20"
    x-axis [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20]
    y-axis "Cumulative Supply (Billion EPIX)" 0 --> 45
    line [10.5, 18.4, 24.3, 28.8, 32.1, 34.6, 36.5, 37.9, 39.0, 39.8, 40.4, 40.8, 41.2, 41.4, 41.6, 41.7, 41.8, 41.9, 41.95, 42.0]
```

### Emission Formula

The module uses smooth per-block exponential decay:

```
current_rate = initial_amount × (1 - reduction_rate)^(blocks_elapsed / blocks_per_year)
tokens_per_block = current_rate / blocks_per_year
```

Where:
- `initial_amount` = 10.527B EPIX
- `reduction_rate` = 0.25 (25%)
- `blocks_per_year` = calculated dynamically from block time

### Emission Distribution

- **98%** to staking rewards (validators and delegators)
- **2%** to community pool

## Parameters

| Parameter | Type | Description | Default |
|-----------|------|-------------|---------|
| `mint_denom` | string | Denomination to mint | "aepix" |
| `initial_annual_mint_amount` | Int | Starting emission amount | 10.527B EPIX |
| `annual_reduction_rate` | Dec | Annual reduction percentage | 0.25 (25%) |
| `block_time_seconds` | uint64 | Expected block time | 6 seconds |
| `max_supply` | Int | Maximum total supply | 42B EPIX |
| `community_pool_rate` | Dec | Community pool allocation | 0.02 (2%) |
| `staking_rewards_rate` | Dec | Staking rewards allocation | 0.98 (98%) |

## Key Features

### 1. Block-Time Agnostic

The system automatically adjusts emission rates based on the configured block time. If governance changes the block time, the per-block emission adjusts to maintain the same annual rate.

### 2. Smooth Exponential Decay

Instead of step-wise annual reductions, the system applies continuous exponential decay on a per-block basis, resulting in perfectly smooth emission curves.

### 3. Maximum Supply Protection

Multiple layers of protection ensure the total supply never exceeds 42B EPIX:
- Pre-mint supply check
- Precise amount calculation to reach exactly 42B
- Zero-amount minting prevention

### 4. Governance Integration

All parameters can be updated via governance proposals using the `MsgUpdateParams` message.

## Queries

### Current Emission Rate
```bash
epixd query epixmint annual-provisions
```

### Current Inflation Rate
```bash
epixd query epixmint inflation
```

### Current Supply
```bash
epixd query epixmint current-supply
```

### Maximum Supply
```bash
epixd query epixmint max-supply
```

### Parameters
```bash
epixd query epixmint params
```

## CLI Commands

### Update Parameters (Governance)
```bash
epixd tx epixmint update-params \
  aepix \
  10527000000000000000000000000 \
  0.25 \
  6 \
  42000000000000000000000000000 \
  0.02 \
  0.98 \
  --from mykey
```

## Migration from Standard Mint Module

The EpixMint module is designed to replace the standard Cosmos SDK mint module:

1. **Disable standard mint**: Set inflation parameters to zero
2. **Enable EpixMint**: Configure with desired parameters
3. **Governance control**: Both modules support parameter updates via governance

## Mathematical Verification

The emission schedule has been mathematically verified to reach approximately 42B EPIX over 20 years:

- **Year 1**: 10.527B EPIX
- **Year 2**: 7.895B EPIX  
- **Year 5**: 3.331B EPIX
- **Year 10**: 563M EPIX
- **Year 20**: 16M EPIX
- **Total**: ~42B EPIX

The maximum supply protection ensures the total never exceeds exactly 42B EPIX.
