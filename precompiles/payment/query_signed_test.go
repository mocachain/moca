package payment_test

import (
	"math/big"
	"testing"

	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/suite"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/ethereum/go-ethereum/accounts/abi"

	"github.com/mocachain/moca/v2/precompiles/payment"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
)

// QuerySignedTestSuite covers the payment precompile queries whose results carry
// signed amounts: a payer's netflow rate is negative, and so can be its static
// balance, frozen netflow rate, dynamic balance and locked fee.
type QuerySignedTestSuite struct {
	SupplyTestSuite
}

func TestQuerySignedTestSuite(t *testing.T) {
	suite.Run(t, new(QuerySignedTestSuite))
}

func (s *QuerySignedTestSuite) TestStreamRecordAndDynamicBalance_NegativeAmounts() {
	s.Require().NoError(s.app.PaymentKeeper.SetParams(s.ctx, paymenttypes.DefaultParams()))
	s.mustEnableStaticPrecompiles()

	elapsed := int64(10)
	record := paymenttypes.StreamRecord{
		Account:           sdk.AccAddress(s.address.Bytes()).String(),
		CrudTimestamp:     s.ctx.BlockTime().Unix() - elapsed,
		NetflowRate:       sdkmath.NewInt(-789_012_000_000),
		StaticBalance:     sdkmath.NewInt(-5),
		BufferBalance:     sdkmath.NewInt(20),
		LockBalance:       sdkmath.NewInt(10),
		Status:            paymenttypes.STREAM_ACCOUNT_STATUS_ACTIVE,
		OutFlowCount:      1,
		FrozenNetflowRate: sdkmath.NewInt(-7),
	}
	s.app.PaymentKeeper.SetStreamRecord(s.ctx, &record)

	bindingABI, err := payment.IPaymentMetaData.GetAbi()
	s.Require().NoError(err)

	var got payment.StreamRecord
	s.call(bindingABI, payment.StreamRecordMethodName, &got, record.Account)
	s.Require().Equal(record.NetflowRate.BigInt(), got.NetflowRate)
	s.Require().Equal(record.StaticBalance.BigInt(), got.StaticBalance)
	s.Require().Equal(record.BufferBalance.BigInt(), got.BufferBalance)
	s.Require().Equal(record.LockBalance.BigInt(), got.LockBalance)
	s.Require().Equal(record.FrozenNetflowRate.BigInt(), got.FrozenNetflowRate)

	var dyn payment.DynamicBalance
	s.call(bindingABI, payment.DynamicBalanceMethodName, &dyn, record.Account)
	wantDynamic := record.StaticBalance.Add(record.NetflowRate.MulRaw(elapsed))
	s.Require().Equal(wantDynamic.BigInt(), dyn.DynamicBalance)
	s.Require().Equal(record.NetflowRate.BigInt(), dyn.ChangeRate)
	// locked fee = lock + buffer + the negative static balance
	s.Require().Equal(big.NewInt(25), dyn.LockedFee)
	s.Require().Equal(record.NetflowRate.BigInt(), dyn.StreamRecord.NetflowRate)
	s.Require().Equal(s.ctx.BlockTime().Unix(), dyn.CurrentTimestamp)
}

// call runs a read-only precompile query and unpacks its single tuple output into out.
func (s *QuerySignedTestSuite) call(bindingABI *abi.ABI, method string, out interface{}, args ...interface{}) {
	input, err := bindingABI.Pack(method, args...)
	s.Require().NoError(err)
	precompileAddr := payment.GetAddress()
	stateDB := statedb.New(s.ctx, s.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.app.EvmKeeper.CallEVMWithData(s.ctx, stateDB, s.address, &precompileAddr, input, false, false, nil)
	s.Require().NoError(err)
	s.Require().False(res.Failed(), "%s reverted: %s", method, res.VmError)

	values, err := bindingABI.Unpack(method, res.Ret)
	s.Require().NoError(err)
	s.Require().Len(values, 1)
	converted := abi.ConvertType(values[0], out)
	s.Require().NotNil(converted)
}
