package app

import (
	"math/big"
	"testing"
	"time"

	storetypes "cosmossdk.io/store/types"
	"github.com/cometbft/cometbft/crypto/tmhash"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
	evmtestutil "github.com/cosmos/evm/testutil"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/contracts"
)

// TestInitChainInstallsStorageNFTContracts starts a chain from a fresh genesis and
// checks that the bucket, object and group NFT contracts are in place, as on every
// upgraded network, and that the storage keeper's mint and burn reach a real token.
func TestInitChainInstallsStorageNFTContracts(t *testing.T) {
	moca := EthSetup(false, nil)
	ctx := withProposer(t, moca, moca.NewContext(false))

	for _, c := range contracts.StorageNFTGenesisContracts() {
		require.Equal(t, crypto.Keccak256Hash(c.Code), moca.EvmKeeper.GetCodeHash(ctx, c.Address), "code hash at %s", c.Address)
		require.Equal(t, c.Code, moca.EvmKeeper.GetCode(ctx, crypto.Keccak256Hash(c.Code)), "code at %s", c.Address)
		for _, s := range c.Storage {
			require.Equal(t, s.Value, moca.EvmKeeper.GetState(ctx, c.Address, s.Key), "slot %s at %s", s.Key, c.Address)
		}
	}

	abi := contracts.ERC721NonTransferableContract.ABI
	owner := common.HexToAddress("0x00000000000000000000000000000000000a11ce")
	tokenID := big.NewInt(7)
	ownerOf := func(ctx sdk.Context) (common.Address, error) {
		res, err := moca.StorageKeeper.CallEVM(ctx, abi, contracts.ObjectControlHubAddress, contracts.ObjectERC721TokenAddress, false, "ownerOf", tokenID)
		if err != nil {
			return common.Address{}, err
		}
		out, err := abi.Unpack("ownerOf", res.Ret)
		require.NoError(t, err)
		return out[0].(common.Address), nil
	}

	_, err := moca.StorageKeeper.CallEVM(ctx, abi, contracts.ObjectControlHubAddress, contracts.ObjectERC721TokenAddress, true, "mint", owner, tokenID)
	require.NoError(t, err)
	got, err := ownerOf(ctx)
	require.NoError(t, err)
	require.Equal(t, owner, got)

	// The contract refuses a second mint of a token, which is why a content update
	// must not mint the object NFT again. A failed EVM call charges the context's
	// whole gas limit, so it runs on its own branch of the state.
	remintCtx, _ := ctx.CacheContext()
	_, err = moca.StorageKeeper.CallEVM(remintCtx.WithGasMeter(storetypes.NewInfiniteGasMeter()), abi,
		contracts.ObjectControlHubAddress, contracts.ObjectERC721TokenAddress, true, "mint", owner, tokenID)
	require.ErrorContains(t, err, "execution reverted")

	_, err = moca.StorageKeeper.CallEVM(ctx, abi, contracts.ObjectControlHubAddress, contracts.ObjectERC721TokenAddress, true, "burn", tokenID)
	require.NoError(t, err)
	_, err = ownerOf(ctx.WithGasMeter(storetypes.NewInfiniteGasMeter()))
	require.Error(t, err, "a burned token has no owner")
}

// TestInstallStorageNFTContractsKeepsExistingCode leaves an address that already has
// code alone, as when a chain restarts from an exported genesis.
func TestInstallStorageNFTContractsKeepsExistingCode(t *testing.T) {
	moca := EthSetup(false, nil)
	ctx := moca.NewContext(false)

	other := []byte{0x60, 0x00}
	otherHash := crypto.Keccak256Hash(other)
	moca.EvmKeeper.SetCode(ctx, otherHash.Bytes(), other)
	moca.EvmKeeper.SetCodeHash(ctx, contracts.BucketERC721TokenAddress.Bytes(), otherHash.Bytes())

	moca.installStorageNFTContracts(ctx)
	require.Equal(t, otherHash, moca.EvmKeeper.GetCodeHash(ctx, contracts.BucketERC721TokenAddress))
}

// withProposer gives ctx a block header whose proposer is a stored validator, which
// the EVM needs to resolve the block's coinbase.
func withProposer(t *testing.T, moca *Moca, ctx sdk.Context) sdk.Context {
	t.Helper()
	privKey := ed25519.GenPrivKey()
	consAddr := privKey.PubKey().Address()
	pkAny, err := codectypes.NewAnyWithValue(privKey.PubKey())
	require.NoError(t, err)
	validator := stakingtypes.Validator{
		OperatorAddress: sdk.AccAddress(common.HexToAddress("0x1111111111111111111111111111111111111111").Bytes()).String(),
		ConsensusPubkey: pkAny,
	}
	require.NoError(t, moca.StakingKeeper.SetValidator(ctx, validator))
	require.NoError(t, moca.StakingKeeper.SetValidatorByConsAddr(ctx, validator))
	header := evmtestutil.NewHeader(1, time.Date(2025, time.January, 10, 0, 0, 0, 0, time.UTC), ctx.ChainID(),
		sdk.ConsAddress(consAddr.Bytes()), tmhash.Sum([]byte("app")), tmhash.Sum([]byte("validators")))
	return ctx.WithBlockHeader(header)
}
