package evmd

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	epixminttypes "github.com/cosmos/evm/x/epixmint/types"
	"github.com/gorilla/mux"
)

// BankSupplyProxyHandler creates a proxy handler that redirects bank supply requests
// to the EpixMint module for display denomination support
func BankSupplyProxyHandler(clientCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		denom := vars["denom"]

		// Handle display denomination conversion
		var targetDenom string
		switch denom {
		case "epix":
			targetDenom = "epix"
		case "aepix":
			targetDenom = "aepix"
		default:
			// For unknown denominations, return not found
			writeErrorResponse(w, http.StatusNotFound, fmt.Sprintf("denomination %s not found", denom))
			return
		}

		// Create EpixMint query client
		queryClient := epixminttypes.NewQueryClient(clientCtx)

		// Query the EpixMint module
		req := &epixminttypes.QuerySupplyOfRequest{
			Denom: targetDenom,
		}

		resp, err := queryClient.SupplyOf(context.Background(), req)
		if err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Convert to bank module response format
		bankResp := banktypes.QuerySupplyOfResponse{
			Amount: sdk.NewCoin(denom, resp.Supply),
		}

		// Return the response in bank module format
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(bankResp); err != nil {
			writeErrorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

// writeErrorResponse writes an error response to the HTTP writer
func writeErrorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	errorResp := map[string]interface{}{
		"code":    status,
		"message": message,
		"details": []interface{}{},
	}
	json.NewEncoder(w).Encode(errorResp)
}

// RegisterBankSupplyProxy registers the bank supply proxy routes
func RegisterBankSupplyProxy(router *mux.Router, clientCtx client.Context) {
	// Register the proxy handler for bank supply queries
	router.HandleFunc("/cosmos/bank/v1beta1/supply/{denom}", BankSupplyProxyHandler(clientCtx)).Methods("GET")
}
