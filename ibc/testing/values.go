/*
This file contains the variables, constants, and default values
used in the testing package and commonly defined in tests.
*/
package ibctesting

import (
	"time"

	"github.com/cometbft/cometbft/crypto/tmhash"

	ibctransfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	connectiontypes "github.com/cosmos/ibc-go/v10/modules/core/03-connection/types"
	commitmenttypes "github.com/cosmos/ibc-go/v10/modules/core/23-commitment/types"
	ibctm "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
	"github.com/cosmos/ibc-go/v10/testing/mock"

	sdkmath "cosmossdk.io/math"
)

const (
	// Default params constants used to create a TM client
	TrustingPeriod     time.Duration = time.Hour * 24 * 7 * 2
	UnbondingPeriod    time.Duration = time.Hour * 24 * 7 * 3
	MaxClockDrift      time.Duration = time.Second * 10
	DefaultDelayPeriod uint64        = 0

	DefaultChannelVersion = mock.Version

	// Application Ports
	TransferPort = ibctransfertypes.ModuleName
	MockPort     = mock.ModuleName
)

var (
	DefaultOpenInitVersion *connectiontypes.Version

	// DefaultTrustLevel sets params variables used to create a TM client
	DefaultTrustLevel = ibctm.DefaultTrustLevel

	DefaultTimeoutTimestampDelta = uint64(time.Hour.Nanoseconds())
	DefaultCoinAmount            = sdkmath.NewInt(100)

	UpgradePath = []string{"upgrade", "upgradedIBCState"}

	ConnectionVersion = connectiontypes.GetCompatibleVersions()[0]

	_ = commitmenttypes.NewMerklePrefix([]byte("ibc"))
	// unusedHash is a placeholder hash used for testing.
	unusedHash = tmhash.Sum([]byte{0x00})
	MerklePath = commitmenttypes.NewMerklePath([]byte("ibc"), []byte(""))
)
