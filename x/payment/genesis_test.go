package payment_test

import (
	"testing"

	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	moduletestutil "github.com/mocachain/moca/v2/testutil/codec"
	"github.com/mocachain/moca/v2/testutil/nullify"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/payment"
	"github.com/mocachain/moca/v2/x/payment/keeper"
	"github.com/mocachain/moca/v2/x/payment/types"
)

// makeKeeper builds a standalone payment keeper backed by an in-memory store,
// the same way x/payment/keeper/keeper_test.go's makePaymentKeeper does. It is
// duplicated here (rather than imported) because that helper lives in the
// keeper_test package, which is not importable from payment_test. It also
// hands back the codec and mock dependency keepers so other _test.go files in
// this package (e.g. module_test.go) can build an AppModule on top of it.
func makeKeeper(t *testing.T) (*keeper.Keeper, sdk.Context, codec.Codec, *types.MockBankKeeper, *types.MockAccountKeeper) {
	encCfg := moduletestutil.MakeTestEncodingConfig(payment.AppModuleBasic{})
	key := storetypes.NewKVStoreKey(types.StoreKey)
	testCtx := testutil.DefaultContextWithDB(t, key, storetypes.NewTransientStoreKey("transient_test"))

	ctrl := gomock.NewController(t)
	bankKeeper := types.NewMockBankKeeper(ctrl)
	accountKeeper := types.NewMockAccountKeeper(ctrl)
	k := keeper.NewKeeper(
		encCfg.Codec,
		key,
		bankKeeper,
		accountKeeper,
		authtypes.NewModuleAddress(types.ModuleName).String(),
	)
	require.NoError(t, k.SetParams(testCtx.Ctx, types.DefaultParams()))

	return k, testCtx.Ctx, encCfg.Codec, bankKeeper, accountKeeper
}

func TestGenesis(t *testing.T) {
	k, ctx, _, _, _ := makeKeeper(t)

	streamAddr := sample.RandAccAddress()
	ownerAddr := sample.RandAccAddress()
	paymentAddr := sample.RandAccAddress()
	settleAddr := sample.RandAccAddress()

	genesisState := types.GenesisState{
		Params:           types.DefaultParams(),
		StreamRecordList: []types.StreamRecord{*types.NewStreamRecord(streamAddr, ctx.BlockTime().Unix())},
		PaymentAccountCountList: []types.PaymentAccountCount{
			{Owner: ownerAddr.String(), Count: 3},
		},
		PaymentAccountList: []types.PaymentAccount{
			{Addr: paymentAddr.String(), Owner: ownerAddr.String(), Refundable: true},
		},
		AutoSettleRecordList: []types.AutoSettleRecord{
			{Timestamp: 12345, Addr: settleAddr.String()},
		},
	}

	payment.InitGenesis(ctx, *k, genesisState)
	got := payment.ExportGenesis(ctx, *k)
	require.NotNil(t, got)

	nullify.Fill(&genesisState)
	nullify.Fill(got)
	require.Equal(t, genesisState, *got)
}

func TestInitGenesis_PanicsOnInvalidParams(t *testing.T) {
	k, ctx, _, _, _ := makeKeeper(t)

	// A zero-value Params fails Params.Validate() (e.g. ReserveTime <=
	// ForcedSettleTime), so SetParams returns an error that InitGenesis must
	// surface as a panic rather than silently starting the chain with
	// unvalidated params.
	genesisState := types.GenesisState{Params: types.Params{}}

	require.Panics(t, func() {
		payment.InitGenesis(ctx, *k, genesisState)
	})
}
