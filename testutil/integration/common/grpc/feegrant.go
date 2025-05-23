package grpc

import (
	"context"
	"cosmossdk.io/x/feegrant"
)

// GetGrants returns the grants for the given grantee and granter combination.
//
// NOTE: To extract the concrete authorizations, use the GetAuthorizations method.
func (gqh *IntegrationHandler) Allowance(grantee, granter string) (*feegrant.QueryAllowanceResponse, error) {
	feegrantClient := gqh.network.GetFeeGrantClient()
	res, err := feegrantClient.Allowance(context.Background(), &feegrant.QueryAllowanceRequest{
		Grantee: grantee,
		Granter: granter,
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}
