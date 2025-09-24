package evmd

import (
	"context"
	"fmt"
	"net/http"

	"cosmossdk.io/math"
	"github.com/gorilla/mux"

	"github.com/cosmos/evm/x/epixmint/types"

	"github.com/cosmos/cosmos-sdk/client"
)

// SupplyAPIHandler creates a handler for simple supply queries compatible with trackers
// Supports query parameters like ?q=totalcoins and ?q=circulatingsupply
func SupplyAPIHandler(clientCtx client.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Get the query parameter
		query := r.URL.Query().Get("q")
		if query == "" {
			writeTextErrorResponse(w, http.StatusBadRequest, "Missing query parameter 'q'")
			return
		}

		// Create EpixMint query client
		queryClient := types.NewQueryClient(clientCtx)

		switch query {
		case "totalcoins", "circulatingsupply":
			// For EpixChain, total supply and circulating supply are the same
			// Get supply in EPIX denomination (display units)
			req := &types.QuerySupplyOfRequest{
				Denom: "epix",
			}

			resp, err := queryClient.SupplyOf(context.Background(), req)
			if err != nil {
				writeTextErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to query supply: %s", err.Error()))
				return
			}

			// Return just the number as plain text
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", resp.Supply.String())

		case "maxsupply":
			// Get maximum supply
			req := &types.QueryMaxSupplyRequest{}

			resp, err := queryClient.MaxSupply(context.Background(), req)
			if err != nil {
				writeTextErrorResponse(w, http.StatusInternalServerError, fmt.Sprintf("Failed to query max supply: %s", err.Error()))
				return
			}

			// Convert from aepix to epix (divide by 10^18)
			conversionFactor := math.NewInt(1000000000000000000) // 10^18
			epixMaxSupply := resp.MaxSupply.Quo(conversionFactor)

			// Return just the number as plain text
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "%s", epixMaxSupply.String())

		default:
			writeTextErrorResponse(w, http.StatusBadRequest, fmt.Sprintf("Unsupported query: %s. Supported queries: totalcoins, circulatingsupply, maxsupply", query))
		}
	}
}

// writeTextErrorResponse writes a plain text error response
func writeTextErrorResponse(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(status)
	fmt.Fprintf(w, "Error: %s", message)
}

// RegisterSupplyAPI registers the supply API routes
func RegisterSupplyAPI(router *mux.Router, clientCtx client.Context) {
	// Register the handler for supply queries
	router.HandleFunc("/api.dws", SupplyAPIHandler(clientCtx)).Methods("GET")
}
