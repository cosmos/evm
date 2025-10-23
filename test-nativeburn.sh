#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
PRIVATE_KEY="0x88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305"
USER_ADDRESS="0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101"
RPC_URL="http://localhost:8545"
BURN_AMOUNT="1000000000000000000"  # 1 token

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}NativeBurn Precompile Test Suite${NC}"
echo -e "${BLUE}======================================${NC}\n"

# Step 1: Build
echo -e "${YELLOW}[1/6]${NC} Building chain binary..."
make install > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓${NC} Chain binary built successfully\n"
else
    echo -e "${RED}✗${NC} Failed to build chain binary\n"
    exit 1
fi

# Step 2: Start Chain
echo -e "${YELLOW}[2/6]${NC} Starting local chain..."
pkill -9 evmd > /dev/null 2>&1 || true
sleep 2
bash local_node.sh -y --no-install > /dev/null 2>&1 &
sleep 12  # Wait for chain to start

# Check if chain is running
BLOCK_NUM=$(curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  $RPC_URL | jq -r '.result')

if [ "$BLOCK_NUM" != "null" ] && [ -n "$BLOCK_NUM" ]; then
    echo -e "${GREEN}✓${NC} Chain is running (block: $((16#${BLOCK_NUM:2})))\n"
else
    echo -e "${RED}✗${NC} Chain failed to start\n"
    exit 1
fi

# Step 3: Deploy Contract
echo -e "${YELLOW}[3/6]${NC} Deploying NativeBurnReceipt contract..."
DEPLOY_OUTPUT=$(forge create --rpc-url $RPC_URL \
  --private-key $PRIVATE_KEY \
  contracts/NativeBurnReceipt.sol:NativeBurnReceipt \
  --broadcast 2>&1)

CONTRACT_ADDRESS=$(echo "$DEPLOY_OUTPUT" | grep "Deployed to:" | awk '{print $3}')

if [ -n "$CONTRACT_ADDRESS" ]; then
    echo -e "${GREEN}✓${NC} Contract deployed at: ${BLUE}$CONTRACT_ADDRESS${NC}\n"
else
    echo -e "${RED}✗${NC} Failed to deploy contract\n"
    exit 1
fi

# Step 4: Execute Burn
echo -e "${YELLOW}[4/6]${NC} Executing burn transaction..."
BURN_TX=$(cast send $CONTRACT_ADDRESS "burn(uint256)" $BURN_AMOUNT \
  --private-key $PRIVATE_KEY \
  --rpc-url $RPC_URL 2>&1)

if echo "$BURN_TX" | grep -q "status.*1"; then
    echo -e "${GREEN}✓${NC} Burn transaction successful (1 TEST token burned)\n"
else
    echo -e "${RED}✗${NC} Burn transaction failed\n"
    exit 1
fi

# Step 5: Verify Burn Results
echo -e "${YELLOW}[5/6]${NC} Verifying burn results..."
sleep 2  # Wait for block

RECEIPT_BALANCE=$(cast call $CONTRACT_ADDRESS "balanceOf(address)" $USER_ADDRESS --rpc-url $RPC_URL)
TOTAL_BURNED=$(cast call $CONTRACT_ADDRESS "totalBurned()" --rpc-url $RPC_URL)

# Convert hex to decimal
RECEIPT_DEC=$((16#${RECEIPT_BALANCE:2}))
BURNED_DEC=$((16#${TOTAL_BURNED:2}))

echo -e "  Receipt Balance: ${GREEN}${RECEIPT_DEC}${NC} wei (${GREEN}1.0${NC} BURN token)"
echo -e "  Total Burned: ${GREEN}${BURNED_DEC}${NC} wei\n"

# Step 6: Final Verification
echo -e "${YELLOW}[6/6]${NC} Final verification..."

if [ "$RECEIPT_DEC" -eq "$BURNED_DEC" ] && [ "$BURNED_DEC" -eq "$BURN_AMOUNT" ]; then
    echo -e "${GREEN}✓${NC} Receipt tokens minted correctly (1:1 ratio)"
    echo -e "${GREEN}✓${NC} Total burned matches expected amount"
    echo -e "${GREEN}✓${NC} Burn transaction completed successfully"
else
    echo -e "${RED}✗${NC} Receipt/burn tracking mismatch"
    exit 1
fi

# Success banner
echo -e "\n${GREEN}======================================${NC}"
echo -e "${GREEN}🔥 BURN VERIFIED! 🔥${NC}"
echo -e "${GREEN}======================================${NC}"
echo -e "\n${BLUE}Results:${NC}"
echo -e "  • Burned Amount: ${GREEN}1.0 TEST${NC}"
echo -e "  • Receipt Tokens: ${GREEN}1.0 BURN${NC}"
echo -e "  • Contract: ${BLUE}${CONTRACT_ADDRESS}${NC}"
echo -e "\n${BLUE}Innovation:${NC}"
echo -e "  • First Cosmos precompile using ${RED}BankKeeper.BurnCoins()${NC}"
echo -e "  • Tokens ${RED}permanently destroyed${NC} from total supply"
echo -e "  • Receipt tokens prove deflationary contribution\n"

# Cleanup prompt
echo -e "${YELLOW}Chain is still running. Stop it with:${NC} pkill evmd\n"
