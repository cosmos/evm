package evmd

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/bank"

	"github.com/cosmos/evm/x/epixmint"
)

func TestSupplyAPIHandler(t *testing.T) {
	// Create a basic codec for testing
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	auth.AppModuleBasic{}.RegisterInterfaces(interfaceRegistry)
	bank.AppModuleBasic{}.RegisterInterfaces(interfaceRegistry)
	epixmint.AppModuleBasic{}.RegisterInterfaces(interfaceRegistry)

	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create a mock client context
	clientCtx := client.Context{}.
		WithCodec(cdc).
		WithInterfaceRegistry(interfaceRegistry)

	// Create router and register the handler
	router := mux.NewRouter()
	RegisterSupplyAPI(router, clientCtx)

	testCases := []struct {
		name           string
		query          string
		expectedStatus int
		expectError    bool
	}{
		{
			name:           "invalid query",
			query:          "invalid",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name:           "empty query",
			query:          "",
			expectedStatus: http.StatusBadRequest,
			expectError:    true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create request
			req, err := http.NewRequest("GET", "/api.dws?q="+tc.query, nil)
			require.NoError(t, err)

			// Create response recorder
			rr := httptest.NewRecorder()

			// Serve the request
			router.ServeHTTP(rr, req)

			// Check status code
			require.Equal(t, tc.expectedStatus, rr.Code)

			// For error cases, check that we get an error message
			if tc.expectError {
				require.Contains(t, rr.Body.String(), "Error:")
			}
		})
	}
}

func TestSupplyAPIHandlerValidQueries(t *testing.T) {
	// Test that valid queries are handled correctly (even if they fail due to no gRPC connection)
	// This tests the routing and basic query parameter parsing

	// Create a basic codec for testing
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	auth.AppModuleBasic{}.RegisterInterfaces(interfaceRegistry)
	bank.AppModuleBasic{}.RegisterInterfaces(interfaceRegistry)
	epixmint.AppModuleBasic{}.RegisterInterfaces(interfaceRegistry)

	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create a mock client context
	clientCtx := client.Context{}.
		WithCodec(cdc).
		WithInterfaceRegistry(interfaceRegistry)

	// Create router and register the handler
	router := mux.NewRouter()
	RegisterSupplyAPI(router, clientCtx)

	validQueries := []string{"totalcoins", "circulatingsupply", "maxsupply"}

	for _, query := range validQueries {
		t.Run("valid_query_"+query, func(t *testing.T) {
			// Create request
			req, err := http.NewRequest("GET", "/api.dws?q="+query, nil)
			require.NoError(t, err)

			// Create response recorder
			rr := httptest.NewRecorder()

			// Serve the request
			router.ServeHTTP(rr, req)

			// Should get an internal server error (500) because gRPC client isn't connected
			// but this proves the routing and query parameter parsing works
			require.Equal(t, http.StatusInternalServerError, rr.Code)
			require.Equal(t, "text/plain", rr.Header().Get("Content-Type"))
			require.Contains(t, rr.Body.String(), "Error:")
		})
	}
}

func TestSupplyAPIHandlerRegistration(t *testing.T) {
	// Create a basic codec for testing
	interfaceRegistry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(interfaceRegistry)
	cdc := codec.NewProtoCodec(interfaceRegistry)

	// Create a mock client context
	clientCtx := client.Context{}.
		WithCodec(cdc).
		WithInterfaceRegistry(interfaceRegistry)

	// Create router and register the handler
	router := mux.NewRouter()
	RegisterSupplyAPI(router, clientCtx)

	// Test that the route is registered
	req, err := http.NewRequest("GET", "/api.dws", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	// Should get a bad request (missing query param) but not 404
	require.Equal(t, http.StatusBadRequest, rr.Code)
	require.NotEqual(t, http.StatusNotFound, rr.Code)
}
