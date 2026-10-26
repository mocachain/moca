package gov_test

import (
	"math/big"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/app"
	"github.com/mocachain/moca/v2/precompiles/gov"
)

// TestSubmitProposalSumsInitialDeposit pins how both submit methods build the
// initial deposit: an unsorted deposit with a repeated denom is summed into one
// coin and a zero entry is dropped before the gov keeper sees it. The deposit is
// in a denom gov does not accept, so the keeper's denom check rejects it and its
// error shows the deposit the precompile built.
func TestSubmitProposalSumsInitialDeposit(t *testing.T) {
	a := app.EthSetup(false, nil)
	ctx := a.NewContext(false)
	caller := common.HexToAddress("0x1111111111111111111111111111111111111111")

	stateDB := statedb.New(ctx, a.EvmKeeper, statedb.NewEmptyTxConfig())
	evm := &vm.EVM{Context: vm.BlockContext{BlockNumber: big.NewInt(1)}, StateDB: stateDB}
	evm.SetTxContext(vm.TxContext{Origin: caller})
	contract := vm.NewContract(caller, gov.GetAddress(), uint256.NewInt(0), 10_000_000, nil)
	p := gov.NewPrecompile(
		govkeeper.NewMsgServerImpl(&a.GovKeeper),
		govkeeper.NewQueryServer(&a.GovKeeper),
		a.AccountKeeper,
		a.BankKeeper,
		a.AppCodec(),
	)

	deposit := []gov.Coin{
		{Denom: "zzzz", Amount: big.NewInt(2)},
		{Denom: "yyyy", Amount: big.NewInt(0)},
		{Denom: "zzzz", Amount: big.NewInt(3)},
	}

	testCases := []struct {
		name   string
		method string
		submit func(sdk.Context, *vm.EVM, *vm.Contract, *abi.Method, []interface{}) ([]byte, error)
		args   []interface{}
	}{
		{"legacySubmitProposal", gov.LegacySubmitProposalMethod, p.LegacySubmitProposal, []interface{}{"title", "description", deposit}},
		{"submitProposal", gov.SubmitProposalMethod, p.SubmitProposal, []interface{}{"[]", deposit, "metadata", "title", "summary", false}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			method := p.Methods[tc.method]
			_, err := tc.submit(ctx, evm, contract, &method, tc.args)
			require.ErrorIs(t, err, govtypes.ErrInvalidDepositDenom)
			require.ErrorContains(t, err, "deposited 5zzzz,")
		})
	}
}
