package namespaces

import (
	"strings"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

const (
	NamespacePersonal = "personal"

	// Personal namespace (deprecated in favor of Clef)
	MethodNamePersonalListAccounts     types.RpcName = "personal_listAccounts"
	MethodNamePersonalDeriveAccount    types.RpcName = "personal_deriveAccount"
	MethodNamePersonalEcRecover        types.RpcName = "personal_ecRecover"
	MethodNamePersonalImportRawKey     types.RpcName = "personal_importRawKey"
	MethodNamePersonalListWallets      types.RpcName = "personal_listWallets"
	MethodNamePersonalNewAccount       types.RpcName = "personal_newAccount"
	MethodNamePersonalOpenWallet       types.RpcName = "personal_openWallet"
	MethodNamePersonalSendTransaction  types.RpcName = "personal_sendTransaction"
	MethodNamePersonalSign             types.RpcName = "personal_sign"
	MethodNamePersonalSignTransaction  types.RpcName = "personal_signTransaction"
	MethodNamePersonalSignTypedData    types.RpcName = "personal_signTypedData"
	MethodNamePersonalUnlockAccount    types.RpcName = "personal_unlockAccount"
	MethodNamePersonalLockAccount      types.RpcName = "personal_lockAccount"
	MethodNamePersonalUnpair           types.RpcName = "personal_unpair"
	MethodNamePersonalInitializeWallet types.RpcName = "personal_initializeWallet"
)

// Personal method handlers
func PersonalListAccounts(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result []string
	err := rCtx.Evmd.RPCClient().Call(&result, "personal_listAccounts")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNamePersonalListAccounts,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespacePersonal,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNamePersonalListAccounts,
		Status:   types.Ok,
		Value:    result,
		Category: NamespacePersonal,
	}, nil
}

// PersonalNewAccount tests personal_newAccount with a passphrase
func PersonalNewAccount(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result string
	err := rCtx.Evmd.RPCClient().Call(&result, "personal_newAccount", "test_passphrase")
	if err != nil {
		// Check for expected security/key management errors
		errMsg := err.Error()
		if strings.Contains(errMsg, "too many failed passphrase attempts") ||
			strings.Contains(errMsg, "passphrase") ||
			strings.Contains(errMsg, "authentication") {
			return &types.RpcResult{
				Method:   MethodNamePersonalNewAccount,
				Status:   types.Legacy,
				Value:    "Personal namespace deprecated - API functional but security restricted: " + errMsg,
				Category: NamespacePersonal,
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNamePersonalNewAccount,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespacePersonal,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNamePersonalNewAccount,
		Status:   types.Legacy,
		Value:    "Personal namespace deprecated - but functional: " + result,
		Category: NamespacePersonal,
	}, nil
}

// PersonalSign tests personal_sign with test data
func PersonalSign(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result string
	testData := "0xdeadbeaf"
	testAccount := "0x7cb61d4117ae31a12e393a1cfa3bac666481d02e" // coinbase address
	err := rCtx.Evmd.RPCClient().Call(&result, "personal_sign", testData, testAccount, "test_passphrase")
	if err != nil {
		// Check for expected key management errors
		errMsg := err.Error()
		if strings.Contains(errMsg, "no key for given address") ||
			strings.Contains(errMsg, "key not found") ||
			strings.Contains(errMsg, "keyring") {
			return &types.RpcResult{
				Method:   MethodNamePersonalSign,
				Status:   types.Legacy,
				Value:    "Personal namespace deprecated - API functional but key management error: " + errMsg,
				Category: NamespacePersonal,
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNamePersonalSign,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespacePersonal,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNamePersonalSign,
		Status:   types.Legacy,
		Value:    "Personal namespace deprecated - but functional: " + result,
		Category: NamespacePersonal,
	}, nil
}

// PersonalImportRawKey tests personal_importRawKey with a test private key
func PersonalImportRawKey(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result string
	testPrivateKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80" // test private key
	err := rCtx.Evmd.RPCClient().Call(&result, "personal_importRawKey", testPrivateKey, "test_passphrase")
	if err != nil {
		// Check for expected security/passphrase errors
		errMsg := err.Error()
		if strings.Contains(errMsg, "too many failed passphrase attempts") ||
			strings.Contains(errMsg, "passphrase") ||
			strings.Contains(errMsg, "authentication") {
			return &types.RpcResult{
				Method:   MethodNamePersonalImportRawKey,
				Status:   types.Legacy,
				Value:    "Personal namespace deprecated - API functional but security restricted: " + errMsg,
				Category: NamespacePersonal,
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNamePersonalImportRawKey,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespacePersonal,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNamePersonalImportRawKey,
		Status:   types.Legacy,
		Value:    "Personal namespace deprecated - but functional: " + result,
		Category: NamespacePersonal,
	}, nil
}

// PersonalSendTransaction tests personal_sendTransaction with test transaction
func PersonalSendTransaction(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result string
	testTx := map[string]interface{}{
		"from":  "0x7cb61d4117ae31a12e393a1cfa3bac666481d02e", // coinbase
		"to":    "0x0100000000000000000000000000000000000000", // test address
		"value": "0x1000",                                     // small amount
		"gas":   "0x5208",                                     // 21000 gas
	}
	err := rCtx.Evmd.RPCClient().Call(&result, "personal_sendTransaction", testTx, "test_passphrase")
	if err != nil {
		// Check for expected key management errors
		errMsg := err.Error()
		if strings.Contains(errMsg, "failed to find key in the node's keyring") ||
			strings.Contains(errMsg, "no key for given address") ||
			strings.Contains(errMsg, "key not found") {
			return &types.RpcResult{
				Method:   MethodNamePersonalSendTransaction,
				Status:   types.Legacy,
				Value:    "Personal namespace deprecated - API functional but key management error: " + errMsg,
				Category: NamespacePersonal,
			}, nil
		}
		return &types.RpcResult{
			Method:   MethodNamePersonalSendTransaction,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespacePersonal,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNamePersonalSendTransaction,
		Status:   types.Legacy,
		Value:    "Personal namespace deprecated - but functional: " + result,
		Category: NamespacePersonal,
	}, nil
}

func PersonalEcRecover(rCtx *types.RPCContext) (*types.RpcResult, error) {
	// Test with known data
	var result string
	err := rCtx.Evmd.RPCClient().Call(&result, "personal_ecRecover",
		"0xdeadbeaf",
		"0xf9ff74c86aefeb5f6019d77280bbb44fb695b4d45cfe97e6eed7acd62905f4a85034d5c68ed25a2e7a8eeb9baf1b8401e4f865d92ec48c1763bf649e354d900b1c")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNamePersonalEcRecover,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespacePersonal,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNamePersonalEcRecover,
		Status:   types.Ok,
		Value:    result,
		Category: NamespacePersonal,
	}, nil
}

func PersonalListWallets(rCtx *types.RPCContext) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.Evmd.RPCClient().Call(&result, "personal_listWallets")
	if err != nil {
		return &types.RpcResult{
			Method:   MethodNamePersonalListWallets,
			Status:   types.Error,
			ErrMsg:   err.Error(),
			Category: NamespacePersonal,
		}, nil
	}
	return &types.RpcResult{
		Method:   MethodNamePersonalListWallets,
		Status:   types.Ok,
		Value:    result,
		Category: NamespacePersonal,
	}, nil
}
