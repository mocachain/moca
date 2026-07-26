package storageprovider

import (
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

const (
	// UpdateSPPriceMethodName is the ABI name for the UpdateSPPrice transaction.
	UpdateSPPriceMethodName = "updateSPPrice"
)

// UpdateSPPrice updates the caller's storage provider read/store price and free read quota.
func (p Precompile) UpdateSPPrice(ctx sdk.Context, evm *vm.EVM, contract *vm.Contract, method *abi.Method, args []interface{}) ([]byte, error) {
	var input UpdateSPPriceArgs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}

	msg := &sptypes.MsgUpdateSpStoragePrice{
		SpAddress:     contract.Caller().String(),
		ReadPrice:     math.LegacyNewDecFromBigIntWithPrec(input.ReadPrice, math.LegacyPrecision),
		FreeReadQuota: input.FreeReadQuota,
		StorePrice:    math.LegacyNewDecFromBigIntWithPrec(input.StorePrice, math.LegacyPrecision),
	}
	if err := msg.ValidateBasic(); err != nil {
		return nil, err
	}

	if _, err := p.spMsgServer.UpdateSpStoragePrice(ctx, msg); err != nil {
		return nil, err
	}

	if err := p.EmitUpdateSPPriceEvent(evm, contract.Caller()); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
