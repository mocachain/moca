package storage_test

// Storage precompile calls that go on to make a nested EVM call (the bucket,
// object and group NFT mint and burn) run that call on a cache context of the
// precompile's own context. These tests pin that a revert after the nested call
// leaves the transaction in a consistent state, and that the bank balances the
// call moves add up.

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/mocachain/moca/v2/precompiles/storage"
	"github.com/mocachain/moca/v2/testutil"
	mocacommon "github.com/mocachain/moca/v2/types/common"
	"github.com/mocachain/moca/v2/utils"
	paymentkeeper "github.com/mocachain/moca/v2/x/payment/keeper"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	storagetypes "github.com/mocachain/moca/v2/x/storage/types"
	vgtypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// NestedEVMCallTestSuite reuses CreateGroupTestSuite's chain setup without
// running that suite's own tests again.
type NestedEVMCallTestSuite struct {
	suite.Suite
	env CreateGroupTestSuite
}

func (s *NestedEVMCallTestSuite) SetupTest() {
	s.env.SetT(s.T())
	s.env.SetupTest()
}

func TestNestedEVMCallTestSuite(t *testing.T) {
	suite.Run(t, new(NestedEVMCallTestSuite))
}

// TestCreateGroup_OutOfGasAroundNestedMint runs createGroup with gas limits that
// run out before, during and after the group NFT mint. Every run must end in a
// clean success or a clean revert that leaves no group behind.
func (s *NestedEVMCallTestSuite) TestCreateGroup_OutOfGasAroundNestedMint() {
	s.env.mustEnableStaticPrecompiles()
	creator := sdk.AccAddress(s.env.address.Bytes())
	precompileAddr := storage.GetAddress()

	var reverted, succeeded int
	for gas := uint64(60_000); gas <= 200_000; gas += 1_000 {
		groupName := fmt.Sprintf("nested-oog-%d", gas)
		ctx, _ := s.env.ctx.CacheContext()
		ctx = ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
		stateDB := statedb.New(ctx, s.env.app.EvmKeeper, statedb.NewEmptyTxConfig())

		nonce, err := s.env.app.AccountKeeper.GetSequence(ctx, creator)
		s.Require().NoError(err)
		msg := core.Message{
			From:       s.env.address,
			To:         &precompileAddr,
			Nonce:      nonce,
			Value:      new(big.Int),
			GasLimit:   gas,
			GasPrice:   new(big.Int),
			GasTipCap:  new(big.Int),
			GasFeeCap:  new(big.Int),
			Data:       s.env.mustPackCreateGroupInput(groupName, ""),
			AccessList: ethtypes.AccessList{},
		}

		var failed bool
		s.Require().NotPanics(func() {
			res, err := s.env.app.EvmKeeper.ApplyMessage(ctx, stateDB, msg, nil, true, false, true)
			failed = err != nil || res.Failed()
		}, "gas limit %d", gas)

		_, found := s.env.app.StorageKeeper.GetGroupInfo(ctx, creator, groupName)
		if failed {
			reverted++
			s.Require().False(found, "gas limit %d: a reverted createGroup must not leave a group", gas)
		} else {
			succeeded++
			s.Require().True(found, "gas limit %d: a successful createGroup must create the group", gas)
		}
	}
	s.Require().NotZero(reverted, "the sweep must reach gas limits that run out")
	s.Require().NotZero(succeeded, "the sweep must reach a gas limit that is enough")
}

// TestCreateBucket_ReadQuotaBankAccounting creates a bucket with a charged read
// quota through the precompile. The payer's bank balance must go down by what
// the payment module account received, and total supply must not change.
func (s *NestedEVMCallTestSuite) TestCreateBucket_ReadQuotaBankAccounting() {
	s.env.mustEnableStaticPrecompiles()

	paramsCtx := s.env.ctx.WithBlockTime(s.env.ctx.BlockTime().Add(-paramsAge))
	s.Require().NoError(s.env.app.StorageKeeper.SetParams(paramsCtx, storagetypes.DefaultParams()))
	s.Require().NoError(s.env.app.PaymentKeeper.SetParams(paramsCtx, paymenttypes.DefaultParams()))
	s.Require().NoError(s.env.app.VirtualgroupKeeper.SetParams(s.env.ctx, vgtypes.DefaultParams()))
	s.env.app.SpKeeper.SetGlobalSpStorePrice(s.env.ctx, sptypes.GlobalSpStorePrice{
		UpdateTimeSec:       s.env.ctx.BlockTime().Add(-paramsAge).Unix(),
		ReadPrice:           sdkmath.LegacyNewDecWithPrec(1, 3),
		PrimaryStorePrice:   sdkmath.LegacyZeroDec(),
		SecondaryStorePrice: sdkmath.LegacyZeroDec(),
	})

	approvalKey, err := ethcrypto.GenerateKey()
	s.Require().NoError(err)
	spOperator := common.HexToAddress("0x4444444444444444444444444444444444444444")
	sp := sptypes.StorageProvider{
		Id:                 1,
		OperatorAddress:    sdk.AccAddress(spOperator.Bytes()).String(),
		FundingAddress:     sdk.AccAddress(spOperator.Bytes()).String(),
		SealAddress:        sdk.AccAddress(spOperator.Bytes()).String(),
		ApprovalAddress:    sdk.AccAddress(ethcrypto.PubkeyToAddress(approvalKey.PublicKey).Bytes()).String(),
		GcAddress:          sdk.AccAddress(spOperator.Bytes()).String(),
		MaintenanceAddress: sdk.AccAddress(spOperator.Bytes()).String(),
		Status:             sptypes.STATUS_IN_SERVICE,
		TotalDeposit:       sdkmath.ZeroInt(),
		Description:        sptypes.Description{Moniker: "sp1"},
	}
	s.env.app.SpKeeper.SetStorageProvider(s.env.ctx, &sp)
	s.env.app.SpKeeper.SetStorageProviderByOperatorAddr(s.env.ctx, &sp)
	s.env.app.VirtualgroupKeeper.SetGVGFamily(s.env.ctx, &vgtypes.GlobalVirtualGroupFamily{
		Id:                    testGVGFamilyID,
		PrimarySpId:           sp.Id,
		VirtualPaymentAddress: sdk.AccAddress(common.HexToAddress("0x5555555555555555555555555555555555555555").Bytes()).String(),
	})

	// A payment account of the creator pays the read quota.
	creator := sdk.AccAddress(s.env.address.Bytes())
	_, err = paymentkeeper.NewMsgServerImpl(s.env.app.PaymentKeeper).CreatePaymentAccount(s.env.ctx,
		&paymenttypes.MsgCreatePaymentAccount{Creator: creator.String()})
	s.Require().NoError(err)
	payer := s.env.app.PaymentKeeper.DerivePaymentAccountAddress(creator, 0)
	s.Require().NoError(testutil.FundAccountWithBaseDenom(s.env.ctx, s.env.app.BankKeeper, payer, 1_000_000_000_000_000_000))
	payerAddr := common.BytesToAddress(payer.Bytes())

	const bucketName = "nested-read-quota"
	approval := storage.Approval{ExpiredHeight: 100, GlobalVirtualGroupFamilyId: testGVGFamilyID}
	msg := &storagetypes.MsgCreateBucket{
		Creator:          s.env.address.String(),
		BucketName:       bucketName,
		Visibility:       storagetypes.VISIBILITY_TYPE_PRIVATE,
		PaymentAddress:   payerAddr.String(),
		PrimarySpAddress: spOperator.String(),
		PrimarySpApproval: &mocacommon.Approval{
			ExpiredHeight:              approval.ExpiredHeight,
			GlobalVirtualGroupFamilyId: approval.GlobalVirtualGroupFamilyId,
		},
		ChargedReadQuota: 1 << 30,
	}
	approval.Sig, err = ethcrypto.Sign(ethcrypto.Keccak256(msg.GetApprovalBytes()), approvalKey)
	s.Require().NoError(err)

	method := storage.GetAbiMethod(storage.CreateBucketMethodName)
	packed, err := method.Inputs.Pack(bucketName, uint8(msg.Visibility), payerAddr, spOperator, approval, msg.ChargedReadQuota)
	s.Require().NoError(err)
	input := append(append([]byte{}, method.ID...), packed...)

	paymentModule := authtypes.NewModuleAddress(paymenttypes.ModuleName)
	payerBefore := s.env.app.BankKeeper.GetBalance(s.env.ctx, payer, utils.BaseDenom).Amount
	moduleBefore := s.env.app.BankKeeper.GetBalance(s.env.ctx, paymentModule, utils.BaseDenom).Amount
	supplyBefore := s.env.app.BankKeeper.GetSupply(s.env.ctx, utils.BaseDenom).Amount

	precompileAddr := storage.GetAddress()
	stateDB := statedb.New(s.env.ctx, s.env.app.EvmKeeper, statedb.NewEmptyTxConfig())
	res, err := s.env.app.EvmKeeper.CallEVMWithData(s.env.ctx, stateDB, s.env.address, &precompileAddr, input, true, false, nil)
	if res != nil && res.Failed() {
		reason, _ := abi.UnpackRevert(res.Ret)
		s.FailNow("createBucket reverted", "%s: %s", res.VmError, reason)
	}
	s.Require().NoError(err)
	_, found := s.env.app.StorageKeeper.GetBucketInfo(s.env.ctx, bucketName)
	s.Require().True(found)

	payerDebit := payerBefore.Sub(s.env.app.BankKeeper.GetBalance(s.env.ctx, payer, utils.BaseDenom).Amount)
	moduleCredit := s.env.app.BankKeeper.GetBalance(s.env.ctx, paymentModule, utils.BaseDenom).Amount.Sub(moduleBefore)
	s.Require().True(moduleCredit.IsPositive(), "the read quota must be paid from the payer's bank balance")
	s.Require().Equal(moduleCredit.String(), payerDebit.String(), "the payer must be debited exactly what the payment module received")
	s.Require().Equal(supplyBefore.String(), s.env.app.BankKeeper.GetSupply(s.env.ctx, utils.BaseDenom).Amount.String(), "total supply must not change")
}
