package namespaces

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

// WebSocket subscription request/response structures
type SubscriptionRequest struct {
	JsonRPC string        `json:"jsonrpc"`
	ID      int           `json:"id"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
}

type SubscriptionResponse struct {
	JsonRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   interface{} `json:"error,omitempty"`
}

type NotificationMessage struct {
	JsonRPC string            `json:"jsonrpc"`
	Method  string            `json:"method"`
	Params  NotificationParam `json:"params"`
}

type NotificationParam struct {
	Subscription string      `json:"subscription"`
	Result       interface{} `json:"result"`
}

// EthSubscribe tests WebSocket subscription functionality
func EthSubscribe(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// Convert HTTP endpoint to WebSocket endpoint
	// Cosmos EVM uses port 8546 for WebSocket, not 8545
	wsURL := strings.Replace(rCtx.Conf.RpcEndpoint, "http://localhost:8545", "ws://localhost:8546", 1)
	wsURL = strings.Replace(wsURL, "https://localhost:8545", "wss://localhost:8546", 1)
	// Fallback for generic conversion
	if wsURL == rCtx.Conf.RpcEndpoint {
		wsURL = strings.Replace(rCtx.Conf.RpcEndpoint, "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
		wsURL = strings.Replace(wsURL, ":8545", ":8546", 1)
	}

	// Test all 4 subscription types
	subscriptionTypes := []struct {
		name        string
		params      []interface{}
		description string
	}{
		{
			name:        "newHeads",
			params:      []interface{}{"newHeads"},
			description: "New block headers subscription",
		},
		{
			name:        "logs",
			params:      []interface{}{"logs", map[string]interface{}{}}, // Empty filter for all logs
			description: "Event logs subscription",
		},
		{
			name:        "newPendingTransactions",
			params:      []interface{}{"newPendingTransactions"},
			description: "Pending transactions subscription",
		},
		{
			name:        "syncing",
			params:      []interface{}{"syncing"},
			description: "Synchronization status subscription",
		},
	}

	var results []string
	var failedTests []string

	for _, subType := range subscriptionTypes {
		success, err := testWebSocketSubscription(wsURL, subType.params, subType.description)
		if success {
			results = append(results, fmt.Sprintf("✓ %s", subType.name))
		} else {
			failedTests = append(failedTests, fmt.Sprintf("✗ %s: %v", subType.name, err))
			results = append(results, fmt.Sprintf("✗ %s", subType.name))
		}
	}

	// Determine overall result
	if len(failedTests) == 0 {
		return &types.RpcResult{
			Method:   MethodNameEthSubscribe,
			Status:   types.Ok,
			Value:    fmt.Sprintf("All 4 subscription types working: %v", results),
			Category: "eth",
		}, nil
	} else if len(failedTests) < len(subscriptionTypes) {
		return &types.RpcResult{
			Method:   MethodNameEthSubscribe,
			Status:   types.Ok,
			Value:    fmt.Sprintf("Partial support (%d/%d): %v", len(subscriptionTypes)-len(failedTests), len(subscriptionTypes), results),
			Category: "eth",
		}, nil
	} else {
		return &types.RpcResult{
			Method:   MethodNameEthSubscribe,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("All subscription types failed: %v", failedTests),
			Category: "eth",
		}, nil
	}
}

// EthUnsubscribe tests WebSocket unsubscription functionality
func EthUnsubscribe(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// Convert HTTP endpoint to WebSocket endpoint
	// Cosmos EVM uses port 8546 for WebSocket, not 8545
	wsURL := strings.Replace(rCtx.Conf.RpcEndpoint, "http://localhost:8545", "ws://localhost:8546", 1)
	wsURL = strings.Replace(wsURL, "https://localhost:8545", "wss://localhost:8546", 1)
	// Fallback for generic conversion
	if wsURL == rCtx.Conf.RpcEndpoint {
		wsURL = strings.Replace(rCtx.Conf.RpcEndpoint, "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
		wsURL = strings.Replace(wsURL, ":8545", ":8546", 1)
	}

	// Test unsubscription by creating a subscription first, then unsubscribing
	success, subscriptionID, err := testWebSocketUnsubscribe(wsURL)
	if success {
		return &types.RpcResult{
			Method:   MethodNameEthUnsubscribe,
			Status:   types.Ok,
			Value:    fmt.Sprintf("Successfully unsubscribed from subscription: %s", subscriptionID),
			Category: "eth",
		}, nil
	} else {
		return &types.RpcResult{
			Method:   MethodNameEthUnsubscribe,
			Status:   types.Error,
			ErrMsg:   fmt.Sprintf("Failed to test unsubscribe: %v", err),
			Category: "eth",
		}, nil
	}
}

// testWebSocketSubscription tests a specific subscription type
func testWebSocketSubscription(wsURL string, params []interface{}, description string) (bool, error) {
	// Parse the WebSocket URL
	u, err := url.Parse(wsURL)
	if err != nil {
		return false, fmt.Errorf("failed to parse WebSocket URL: %v", err)
	}

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return false, fmt.Errorf("failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Set connection timeout
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	// Send subscription request
	request := SubscriptionRequest{
		JsonRPC: "2.0",
		ID:      1,
		Method:  "eth_subscribe",
		Params:  params,
	}

	if err := conn.WriteJSON(request); err != nil {
		return false, fmt.Errorf("failed to send subscription request: %v", err)
	}

	// Read response
	var response SubscriptionResponse
	if err := conn.ReadJSON(&response); err != nil {
		return false, fmt.Errorf("failed to read subscription response: %v", err)
	}

	// Check if subscription was successful
	if response.Error != nil {
		return false, fmt.Errorf("subscription failed: %v", response.Error)
	}

	if response.Result == nil {
		return false, fmt.Errorf("no subscription ID returned")
	}

	// Subscription was successful
	return true, nil
}

// testWebSocketUnsubscribe tests unsubscription functionality
func testWebSocketUnsubscribe(wsURL string) (bool, string, error) {
	// Parse the WebSocket URL
	u, err := url.Parse(wsURL)
	if err != nil {
		return false, "", fmt.Errorf("failed to parse WebSocket URL: %v", err)
	}

	// Connect to WebSocket
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return false, "", fmt.Errorf("failed to connect to WebSocket: %v", err)
	}
	defer conn.Close()

	// Set connection timeout
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	conn.SetWriteDeadline(time.Now().Add(5 * time.Second))

	// First, create a subscription
	subscribeRequest := SubscriptionRequest{
		JsonRPC: "2.0",
		ID:      1,
		Method:  "eth_subscribe",
		Params:  []interface{}{"newHeads"}, // Use newHeads as test subscription
	}

	if err := conn.WriteJSON(subscribeRequest); err != nil {
		return false, "", fmt.Errorf("failed to send subscription request: %v", err)
	}

	// Read subscription response
	var subscribeResponse SubscriptionResponse
	if err := conn.ReadJSON(&subscribeResponse); err != nil {
		return false, "", fmt.Errorf("failed to read subscription response: %v", err)
	}

	if subscribeResponse.Error != nil {
		return false, "", fmt.Errorf("subscription failed: %v", subscribeResponse.Error)
	}

	subscriptionID, ok := subscribeResponse.Result.(string)
	if !ok {
		return false, "", fmt.Errorf("invalid subscription ID type")
	}

	// Now test unsubscription
	unsubscribeRequest := SubscriptionRequest{
		JsonRPC: "2.0",
		ID:      2,
		Method:  "eth_unsubscribe",
		Params:  []interface{}{subscriptionID},
	}

	if err := conn.WriteJSON(unsubscribeRequest); err != nil {
		return false, subscriptionID, fmt.Errorf("failed to send unsubscribe request: %v", err)
	}

	// Read unsubscribe response
	var unsubscribeResponse SubscriptionResponse
	if err := conn.ReadJSON(&unsubscribeResponse); err != nil {
		return false, subscriptionID, fmt.Errorf("failed to read unsubscribe response: %v", err)
	}

	if unsubscribeResponse.Error != nil {
		return false, subscriptionID, fmt.Errorf("unsubscribe failed: %v", unsubscribeResponse.Error)
	}

	// Check if unsubscribe returned true
	result, ok := unsubscribeResponse.Result.(bool)
	if !ok {
		return false, subscriptionID, fmt.Errorf("invalid unsubscribe result type")
	}

	if !result {
		return false, subscriptionID, fmt.Errorf("unsubscribe returned false")
	}

	return true, subscriptionID, nil
}

// Helper function to test if WebSocket endpoint is available
func IsWebSocketAvailable(rCtx *types.RPCContext) bool {
	// Convert HTTP endpoint to WebSocket endpoint
	// Cosmos EVM uses port 8546 for WebSocket, not 8545
	wsURL := strings.Replace(rCtx.Conf.RpcEndpoint, "http://localhost:8545", "ws://localhost:8546", 1)
	wsURL = strings.Replace(wsURL, "https://localhost:8545", "wss://localhost:8546", 1)
	// Fallback for generic conversion
	if wsURL == rCtx.Conf.RpcEndpoint {
		wsURL = strings.Replace(rCtx.Conf.RpcEndpoint, "http://", "ws://", 1)
		wsURL = strings.Replace(wsURL, "https://", "wss://", 1)
		wsURL = strings.Replace(wsURL, ":8545", ":8546", 1)
	}

	u, err := url.Parse(wsURL)
	if err != nil {
		return false
	}

	conn, _, err := websocket.DefaultDialer.DialContext(context.Background(), u.String(), nil)
	if err != nil {
		return false
	}
	defer conn.Close()

	return true
}
