package keeper

import (
	"github.com/mocachain/moca/v2/x/payment/types"
)

var _ types.QueryServer = Keeper{}
