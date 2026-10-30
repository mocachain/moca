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
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/mocachain/moca/v2/contracts"
	"github.com/mocachain/moca/v2/internal/sequence"
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
// clean success or a clean revert that leaves no group, group ID or token behind.
func (s *NestedEVMCallTestSuite) TestCreateGroup_OutOfGasAroundNestedMint() {
	s.env.mustEnableStaticPrecompiles()
	s.requireGroupNFTContract()
	before := s.groupState(s.env.ctx)

	var reverted, succeeded int
	for gas := uint64(60_000); gas <= 400_000; gas += 1_000 {
		groupName := fmt.Sprintf("nested-oog-%d", gas)
		ctx := s.txContext()
		res, failed := s.applyCreateGroup(ctx, groupName, gas)
		if failed {
			reverted++
			s.requireNoGroupCreated(ctx, before, groupName, res)
		} else {
			succeeded++
			s.requireGroupCreated(ctx, before, groupName, res)
		}
	}
	s.Require().NotZero(reverted, "the sweep must reach gas limits that run out")
	s.Require().NotZero(succeeded, "the sweep must reach a gas limit that is enough")
}

// TestCreateGroup_OutOfGasInAndAfterNestedMint runs out of gas at fixed points of
// createGroup: while the nested call loads the group NFT contract and its state,
// halfway through the nested call writing the minted token, while charging the
// EVM gas of the completed mint, and after the mint, where the precompile reads
// the new group back. The precompile's gas meter is charged the nested call's
// store reads and writes, then its EVM gas, then the read-back, in that order.
// Each limit is placed in the middle of one of those spans, measured on their own
// below. Every run must leave both group indexes, the group ID sequence, the token,
// the logs and the events as they were.
func (s *NestedEVMCallTestSuite) TestCreateGroup_OutOfGasInAndAfterNestedMint() {
	s.env.mustEnableStaticPrecompiles()
	s.requireGroupNFTContract()

	const groupName = "nested-oog-pinned"
	before := s.groupState(s.env.ctx)
	groupID := before.seq.Incr()

	// The least gas limit with which createGroup succeeds.
	succeeds := func(gas uint64) bool {
		_, failed := s.applyCreateGroup(s.txContext(), groupName, gas)
		return !failed
	}
	failing, need := uint64(0), uint64(1_000_000)
	s.Require().True(succeeds(need))
	for need-failing > 1 {
		mid := failing + (need-failing)/2
		if succeeds(mid) {
			need = mid
		} else {
			failing = mid
		}
	}

	// The nested mint as the keeper makes it, and the read-back of the new group.
	mint := s.mintGroupNFT(groupID)
	created := s.txContext()
	_, failed := s.applyCreateGroup(created, groupName, need)
	s.Require().False(failed)
	readBack := created.WithGasMeter(storetypes.NewInfiniteGasMeter())
	_, found := s.env.app.StorageKeeper.GetGroupInfo(readBack, sdk.AccAddress(s.env.address.Bytes()), groupName)
	s.Require().True(found)
	readBackGas := readBack.GasMeter().GasConsumed()

	s.T().Logf("createGroup needs %d gas; nested mint: load %d, write %d, after writes %d, EVM %d; read-back %d",
		need, mint.load, mint.write, mint.settle, mint.evm, readBackGas)
	s.Require().NotZero(mint.load)
	s.Require().NotZero(mint.write)
	s.Require().NotZero(mint.evm)
	s.Require().NotZero(readBackGas)

	evmStart := need - readBackGas - mint.evm
	writeStart := evmStart - mint.settle - mint.write
	cases := []struct {
		name string
		gas  uint64
	}{
		{"loading the NFT contract", writeStart - mint.load/2},
		{"writing the minted token", writeStart + mint.write/2},
		{"charging the mint's EVM gas", evmStart + mint.evm/2},
		{"reading the group back after the mint", need - 1},
	}
	for _, tc := range cases {
		ctx := s.txContext()
		res, failed := s.applyCreateGroup(ctx, groupName, tc.gas)
		s.Require().True(failed, "%s (gas %d) must fail", tc.name, tc.gas)
		s.Require().Contains(res.VmError, vm.ErrOutOfGas.Error(), tc.name)
		s.requireNoGroupCreated(ctx, before, groupName, res)
	}

	ctx := s.txContext()
	res, failed := s.applyCreateGroup(ctx, groupName, need)
	s.Require().False(failed)
	s.requireGroupCreated(ctx, before, groupName, res)
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
	packed, err := method.Inputs.Pack(bucketName, uint8(storagetypes.VISIBILITY_TYPE_PRIVATE), payerAddr, spOperator, approval, msg.ChargedReadQuota)
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

// groupState is the part of the chain a createGroup writes besides the group itself.
type groupState struct {
	seq        sdkmath.Uint // last group ID handed out
	nftSupply  *big.Int
	nftBalance *big.Int // group tokens the test account holds
}

func (s *NestedEVMCallTestSuite) groupState(ctx sdk.Context) groupState {
	store := ctx.KVStore(s.env.app.GetKey(storagetypes.StoreKey))
	supply, err := s.callGroupNFT(ctx, "totalSupply")
	s.Require().NoError(err)
	balance, err := s.callGroupNFT(ctx, "balanceOf", s.env.address)
	s.Require().NoError(err)
	return groupState{
		seq:        sequence.NewSequence[sdkmath.Uint](storagetypes.GroupSequencePrefix).CurVal(store),
		nftSupply:  supply.(*big.Int),
		nftBalance: balance.(*big.Int),
	}
}

// requireGroupNFTContract asserts that the group NFT address holds the contract
// installed at genesis, so createGroup's mint runs real code.
func (s *NestedEVMCallTestSuite) requireGroupNFTContract() {
	stateDB := statedb.New(s.env.ctx, s.env.app.EvmKeeper, statedb.NewEmptyTxConfig())
	code := stateDB.GetCode(contracts.GroupERC721TokenAddress)
	s.Require().NotEmpty(code, "the group NFT address has no code")
	for _, c := range contracts.StorageNFTGenesisContracts() {
		if c.Address == contracts.GroupERC721TokenAddress {
			s.Require().Equal(c.Code, code)
			return
		}
	}
	s.FailNow("no genesis contract at the group NFT address")
}

// txContext returns a branch of the chain for one transaction, with its own event manager.
func (s *NestedEVMCallTestSuite) txContext() sdk.Context {
	ctx, _ := s.env.ctx.CacheContext()
	return ctx.WithGasMeter(storetypes.NewInfiniteGasMeter()).WithEventManager(sdk.NewEventManager())
}

// applyCreateGroup sends createGroup to the storage precompile in an EVM
// transaction from the test account with the given gas limit.
func (s *NestedEVMCallTestSuite) applyCreateGroup(ctx sdk.Context, groupName string, gas uint64) (*evmtypes.MsgEthereumTxResponse, bool) {
	precompileAddr := storage.GetAddress()
	stateDB := statedb.New(ctx, s.env.app.EvmKeeper, statedb.NewEmptyTxConfig())
	nonce, err := s.env.app.AccountKeeper.GetSequence(ctx, sdk.AccAddress(s.env.address.Bytes()))
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

	var res *evmtypes.MsgEthereumTxResponse
	s.Require().NotPanics(func() {
		res, err = s.env.app.EvmKeeper.ApplyMessage(ctx, stateDB, msg, nil, true, false, true)
	}, "gas limit %d", gas)
	s.Require().NoError(err, "gas limit %d", gas)
	return res, res.Failed()
}

// nestedMintGas splits the gas the nested mint charges to its caller's context,
// in the order it is charged.
type nestedMintGas struct {
	load   uint64 // store gas before the first write: the contract, its state, the accounts
	write  uint64 // store gas from the first write to the last
	settle uint64 // store gas after the last write
	evm    uint64 // the call's EVM gas, charged last
}

// gasRecorder is a gas meter that records each charge in order.
type gasRecorder struct {
	storetypes.GasMeter
	charges []gasCharge
}

type gasCharge struct {
	descriptor string
	amount     uint64
}

func (r *gasRecorder) ConsumeGas(amount storetypes.Gas, descriptor string) {
	r.charges = append(r.charges, gasCharge{descriptor: descriptor, amount: amount})
	r.GasMeter.ConsumeGas(amount, descriptor)
}

// mintGroupNFT mints token id to the test account on a branch of the chain, as
// the keeper does in createGroup, and splits the gas it charges.
func (s *NestedEVMCallTestSuite) mintGroupNFT(id sdkmath.Uint) nestedMintGas {
	meter := &gasRecorder{GasMeter: storetypes.NewInfiniteGasMeter()}
	branch, _ := s.env.ctx.CacheContext()
	res, err := s.env.app.StorageKeeper.CallEVM(branch.WithGasMeter(meter), contracts.ERC721NonTransferableContract.ABI,
		contracts.GroupControlHubAddress, contracts.GroupERC721TokenAddress, true, "mint", s.env.address, id.BigInt())
	s.Require().NoError(err)

	charges := meter.charges
	s.Require().NotEmpty(charges)
	g := nestedMintGas{evm: charges[len(charges)-1].amount}
	s.Require().Equal(res.GasUsed, g.evm, "the EVM gas must be charged last")
	charges = charges[:len(charges)-1]

	first, last := -1, -1
	for i, c := range charges {
		switch c.descriptor {
		case storetypes.GasWriteCostFlatDesc, storetypes.GasWritePerByteDesc, storetypes.GasDeleteDesc:
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	s.Require().GreaterOrEqual(first, 0, "the mint must write")
	for i, c := range charges {
		switch {
		case i < first:
			g.load += c.amount
		case i <= last:
			g.write += c.amount
		default:
			g.settle += c.amount
		}
	}
	return g
}

// requireNoGroupCreated asserts that a failed createGroup left no trace: no group
// under its name or under the next ID, the ID sequence where it was, no token, and
// no logs or events.
func (s *NestedEVMCallTestSuite) requireNoGroupCreated(ctx sdk.Context, before groupState, groupName string, res *evmtypes.MsgEthereumTxResponse) {
	groupID := before.seq.Incr()
	_, found := s.env.app.StorageKeeper.GetGroupInfo(ctx, sdk.AccAddress(s.env.address.Bytes()), groupName)
	s.Require().False(found, "%s: group left under its name", groupName)
	_, found = s.env.app.StorageKeeper.GetGroupInfoById(ctx, groupID)
	s.Require().False(found, "%s: group left under ID %s", groupName, groupID)
	s.Require().Equal(before, s.groupState(ctx), groupName)
	exists, err := s.callGroupNFT(ctx, "exists", groupID.BigInt())
	s.Require().NoError(err)
	s.Require().False(exists.(bool), "%s: token %s left", groupName, groupID)
	s.Require().Empty(res.Logs, groupName)
	s.Require().False(hasCreateGroupEvent(ctx), "%s: EventCreateGroup left", groupName)
}

// requireGroupCreated asserts that createGroup created the group under the next
// ID, indexed by name and by ID, and minted its token to the creator.
func (s *NestedEVMCallTestSuite) requireGroupCreated(ctx sdk.Context, before groupState, groupName string, res *evmtypes.MsgEthereumTxResponse) {
	groupID := before.seq.Incr()
	byName, found := s.env.app.StorageKeeper.GetGroupInfo(ctx, sdk.AccAddress(s.env.address.Bytes()), groupName)
	s.Require().True(found, groupName)
	s.Require().Equal(groupID, byName.Id, groupName)
	byID, found := s.env.app.StorageKeeper.GetGroupInfoById(ctx, groupID)
	s.Require().True(found, groupName)
	s.Require().Equal(groupName, byID.GroupName)

	after := s.groupState(ctx)
	s.Require().Equal(groupID, after.seq)
	s.Require().Equal(new(big.Int).Add(before.nftSupply, big.NewInt(1)), after.nftSupply)
	s.Require().Equal(new(big.Int).Add(before.nftBalance, big.NewInt(1)), after.nftBalance)
	owner, err := s.callGroupNFT(ctx, "ownerOf", groupID.BigInt())
	s.Require().NoError(err)
	s.Require().Equal(s.env.address, owner)
	s.Require().NotEmpty(res.Logs, groupName)
	s.Require().True(hasCreateGroupEvent(ctx), groupName)
}

// callGroupNFT reads from the group NFT contract on a branch of ctx.
func (s *NestedEVMCallTestSuite) callGroupNFT(ctx sdk.Context, method string, args ...interface{}) (interface{}, error) {
	branch, _ := ctx.CacheContext()
	contractABI := contracts.ERC721NonTransferableContract.ABI
	res, err := s.env.app.StorageKeeper.CallEVM(branch.WithGasMeter(storetypes.NewInfiniteGasMeter()), contractABI,
		contracts.GroupControlHubAddress, contracts.GroupERC721TokenAddress, false, method, args...)
	if err != nil {
		return nil, err
	}
	out, err := contractABI.Unpack(method, res.Ret)
	s.Require().NoError(err)
	return out[0], nil
}

func hasCreateGroupEvent(ctx sdk.Context) bool {
	eventType := proto.MessageName(&storagetypes.EventCreateGroup{})
	for _, ev := range ctx.EventManager().Events() {
		if ev.Type == eventType {
			return true
		}
	}
	return false
}
