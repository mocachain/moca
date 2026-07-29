package bank_test

import (
	"math/big"

	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	"github.com/mocachain/moca/v2/precompiles/bank"
	"github.com/mocachain/moca/v2/utils"
)

// TestBankSend_OutOfGasSurfacesAsOutOfGas pins the cosmos/evm v0.6.1 (#1049)
// precompile gas semantics through a moca precompile: exhausting the frame's
// gas mid-action must surface raw vm.ErrOutOfGas — which makes evm.Call treat
// it as a real out-of-gas and consume the frame's entire gas — instead of the
// pre-v0.6.1 ABI-encoded revert (vm.ErrExecutionReverted) that refunded the
// remainder to the caller. State must be untouched.
func (s *PrecompileTestSuite) TestBankSend_OutOfGasSurfacesAsOutOfGas() {
	receiver := common.HexToAddress("0x4444444444444444444444444444444444444444")

	// Fresh meter so the precompile's frame meter starts from zero and the
	// 10k frame budget is exhausted by the send's store writes themselves.
	s.ctx = s.ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
	supplyBefore := s.app.BankKeeper.GetSupply(s.ctx, utils.BaseDenom).Amount

	contract := vm.NewContract(s.address, bank.GetAddress(), uint256.NewInt(0), 10_000, nil)
	contract.Input = s.mustPackBankSendInput(receiver, big.NewInt(400_000))

	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	evm := &vm.EVM{Context: vm.BlockContext{BlockNumber: big.NewInt(1)}, StateDB: stateDB}
	evm.SetTxContext(vm.TxContext{Origin: s.address})

	c := bank.NewPrecompile(bankkeeper.NewMsgServerImpl(s.app.BankKeeper, s.app.PaymentKeeper), s.app.BankKeeper)
	ret, err := c.Run(evm, contract, false)

	s.Require().ErrorIs(err, vm.ErrOutOfGas)
	s.Require().NotErrorIs(err, vm.ErrExecutionReverted)
	s.Require().Nil(ret, "out-of-gas must not carry ABI-encoded revert data")

	s.Require().True(s.app.BankKeeper.GetBalance(s.ctx, sdk.AccAddress(receiver.Bytes()), utils.BaseDenom).Amount.IsZero())
	s.Require().Equal(supplyBefore.String(), s.app.BankKeeper.GetSupply(s.ctx, utils.BaseDenom).Amount.String())
}
