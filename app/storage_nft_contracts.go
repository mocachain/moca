package app

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/mocachain/moca/v2/contracts"
)

// installStorageNFTContracts installs the bucket, object and group NFT contracts the
// storage keeper mints and burns through, as moca's pre-v2 EVM module did at genesis,
// so a chain started from a fresh genesis runs the same contracts as an upgraded one.
// An address that already has code, as in a genesis exported from a running chain,
// is left as it is. The control hub the keeper mints and burns from must be an
// account for the EVM to call from it, so it is created when the genesis lacks it.
func (app *Moca) installStorageNFTContracts(ctx sdk.Context) {
	for _, hub := range []common.Address{
		contracts.BucketControlHubAddress, contracts.ObjectControlHubAddress, contracts.GroupControlHubAddress,
	} {
		app.ensureAccount(ctx, hub)
	}
	for _, c := range contracts.StorageNFTGenesisContracts() {
		if !evmtypes.IsEmptyCodeHash(app.EvmKeeper.GetCodeHash(ctx, c.Address).Bytes()) {
			continue
		}
		app.ensureAccount(ctx, c.Address)
		codeHash := crypto.Keccak256Hash(c.Code)
		app.EvmKeeper.SetCode(ctx, codeHash.Bytes(), c.Code)
		app.EvmKeeper.SetCodeHash(ctx, c.Address.Bytes(), codeHash.Bytes())
		for _, s := range c.Storage {
			app.EvmKeeper.SetState(ctx, c.Address, s.Key, s.Value.Bytes())
		}
	}
}

func (app *Moca) ensureAccount(ctx sdk.Context, addr common.Address) {
	accAddr := sdk.AccAddress(addr.Bytes())
	if app.AccountKeeper.GetAccount(ctx, accAddr) == nil {
		app.AccountKeeper.SetAccount(ctx, app.AccountKeeper.NewAccountWithAddress(ctx, accAddr))
	}
}
