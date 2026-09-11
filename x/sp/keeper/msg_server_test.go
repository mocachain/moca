package keeper_test

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"cosmossdk.io/math"
	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/cometbft/cometbft/crypto/tmhash"
	"github.com/cometbft/cometbft/votepool"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	gov "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/sdk/types"
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/sp/keeper"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
)

func (s *KeeperTestSuite) TestMsgCreateStorageProvider() {
	govAddr := authtypes.NewModuleAddress(gov.ModuleName)
	// 1. create new newStorageProvider and grant

	operatorAddr, _, err := testutil.GenerateCoinKey(hd.Secp256k1, s.cdc)
	s.Require().Nil(err, "error should be nil")
	fundingAddr, _, err := testutil.GenerateCoinKey(hd.Secp256k1, s.cdc)
	s.Require().Nil(err, "error should be nil")
	sealAddr, _, err := testutil.GenerateCoinKey(hd.Secp256k1, s.cdc)
	s.Require().Nil(err, "error should be nil")
	approvalAddr, _, err := testutil.GenerateCoinKey(hd.Secp256k1, s.cdc)
	s.Require().Nil(err, "error should be nil")
	gcAddr, _, err := testutil.GenerateCoinKey(hd.Secp256k1, s.cdc)
	s.Require().Nil(err, "error should be nil")
	maintenanceAddr, _, err := testutil.GenerateCoinKey(hd.Secp256k1, s.cdc)
	s.Require().Nil(err, "error should be nil")

	blsPubKeyHex := sample.RandBlsPubKeyHex()

	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), fundingAddr).Return(authtypes.NewBaseAccountWithAddress(fundingAddr)).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	testCases := []struct {
		Name      string
		ExceptErr bool
		req       types.MsgCreateStorageProvider
	}{
		{
			Name:      "invalid funding address",
			ExceptErr: true,
			req: types.MsgCreateStorageProvider{
				Creator: govAddr.String(),
				Description: sptypes.Description{
					Moniker:  "sp_test",
					Identity: "",
				},
				SpAddress:          operatorAddr.String(),
				FundingAddress:     sample.RandAccAddressHex(),
				SealAddress:        sealAddr.String(),
				ApprovalAddress:    approvalAddr.String(),
				GcAddress:          gcAddr.String(),
				MaintenanceAddress: maintenanceAddr.String(),
				BlsKey:             blsPubKeyHex,
				Deposit: sdk.Coin{
					Denom:  types.Denom,
					Amount: types.NewIntFromInt64WithDecimal(10000, types.DecimalMOCA),
				},
			},
		},
		{
			Name:      "invalid endpoint",
			ExceptErr: true,
			req: types.MsgCreateStorageProvider{
				Creator: govAddr.String(),
				Description: sptypes.Description{
					Moniker:  "sp_test",
					Identity: "",
				},
				SpAddress:          operatorAddr.String(),
				FundingAddress:     fundingAddr.String(),
				SealAddress:        sealAddr.String(),
				ApprovalAddress:    approvalAddr.String(),
				GcAddress:          gcAddr.String(),
				MaintenanceAddress: maintenanceAddr.String(),
				BlsKey:             blsPubKeyHex,
				Endpoint:           "sp.io",
				Deposit: sdk.Coin{
					Denom:  types.Denom,
					Amount: types.NewIntFromInt64WithDecimal(10000, types.DecimalMOCA),
				},
			},
		},
		{
			Name:      "invalid bls pub key",
			ExceptErr: true,
			req: types.MsgCreateStorageProvider{
				Creator: govAddr.String(),
				Description: sptypes.Description{
					Moniker:  "sp_test",
					Identity: "",
				},
				SpAddress:          operatorAddr.String(),
				FundingAddress:     fundingAddr.String(),
				SealAddress:        sealAddr.String(),
				ApprovalAddress:    approvalAddr.String(),
				GcAddress:          gcAddr.String(),
				MaintenanceAddress: maintenanceAddr.String(),
				BlsKey:             "InValidBlsPubkey",
				Endpoint:           "sp.io",
				Deposit: sdk.Coin{
					Denom:  types.Denom,
					Amount: types.NewIntFromInt64WithDecimal(10000, types.DecimalMOCA),
				},
			},
		},
		{
			Name:      "success",
			ExceptErr: true,
			req: types.MsgCreateStorageProvider{
				Creator: govAddr.String(),
				Description: sptypes.Description{
					Moniker:  "MsgServer_sp_test",
					Identity: "",
				},
				SpAddress:          operatorAddr.String(),
				FundingAddress:     fundingAddr.String(),
				SealAddress:        sealAddr.String(),
				ApprovalAddress:    approvalAddr.String(),
				GcAddress:          gcAddr.String(),
				MaintenanceAddress: maintenanceAddr.String(),
				BlsKey:             blsPubKeyHex,
				Deposit: sdk.Coin{
					Denom:  types.Denom,
					Amount: types.NewIntFromInt64WithDecimal(10000, types.DecimalMOCA),
				},
			},
		},
	}
	for _, testCase := range testCases {
		s.Suite.T().Run(testCase.Name, func(t *testing.T) {
			req := testCase.req
			_, err := s.msgServer.CreateStorageProvider(s.ctx, &req)
			if testCase.ExceptErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestIsLastDaysOfTheMonth() {
	s.Require().True(!keeper.IsLastDaysOfTheMonth(time.Unix(1693242061, 0), 2)) // 2023-08-28 UTC
	s.Require().True(!keeper.IsLastDaysOfTheMonth(time.Unix(1693328461, 0), 2)) // 2023-08-29 UTC
	s.Require().True(keeper.IsLastDaysOfTheMonth(time.Unix(1693414861, 0), 2))  // 2023-08-30 UTC
	s.Require().True(!keeper.IsLastDaysOfTheMonth(time.Unix(1693587661, 0), 2)) // 2023-09-01 UTC
}

// Helper: generate BLS key and a valid proof over its pubkey bytes
func newTestBlsKeyAndProof() (pubKeyHex string, proofHex string, err error) {
	privKey, err := bls.GenerateBlsKey()
	if err != nil {
		return "", "", err
	}
	pub := privKey.PublicKey().Marshal()
	msgHash := tmhash.Sum(pub)
	sig, err := privKey.Sign(msgHash, votepool.DST)
	if err != nil {
		return "", "", err
	}
	sigBytes, err := sig.Marshal()
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(pub), hex.EncodeToString(sigBytes), nil
}

// Helper: create a storage provider and return its stored record
func (s *KeeperTestSuite) createTestSP(opAddr sdk.AccAddress) *sptypes.StorageProvider {
	fundingAddr := sample.RandAccAddress()
	sealAddr := sample.RandAccAddress()
	approvalAddr := sample.RandAccAddress()
	gcAddr := sample.RandAccAddress()
	maintenanceAddr := sample.RandAccAddress()

	blsPubHex, blsProofHex, err := newTestBlsKeyAndProof()
	s.Require().NoError(err)

	// mocks
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), fundingAddr).Return(authtypes.NewBaseAccountWithAddress(fundingAddr)).AnyTimes()
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	msg := sptypes.MsgCreateStorageProvider{
		Creator:            opAddr.String(), // blockHeight==0, signer must be operator
		SpAddress:          opAddr.String(),
		FundingAddress:     fundingAddr.String(),
		SealAddress:        sealAddr.String(),
		ApprovalAddress:    approvalAddr.String(),
		GcAddress:          gcAddr.String(),
		MaintenanceAddress: maintenanceAddr.String(),
		BlsKey:             blsPubHex,
		BlsProof:           blsProofHex,
		Endpoint:           "https://sp.example",
		Deposit: sdk.Coin{
			Denom:  types.Denom,
			Amount: types.NewIntFromInt64WithDecimal(10000, types.DecimalMOCA),
		},
	}
	_, err = s.msgServer.CreateStorageProvider(s.ctx, &msg)
	s.Require().NoError(err)

	sp, found := s.spKeeper.GetStorageProviderByOperatorAddr(s.ctx, opAddr)
	s.Require().True(found)
	return sp
}

func (s *KeeperTestSuite) TestMsgEditStorageProvider_Uniqueness() {
	// Create two SPs
	spA := s.createTestSP(sample.RandAccAddress())
	spB := s.createTestSP(sample.RandAccAddress())

	// New unique values
	newSeal := sample.RandAccAddress()
	newBlsPubHex, newBlsProofHex, err := newTestBlsKeyAndProof()
	s.Require().NoError(err)

	tests := []struct {
		name      string
		req       *sptypes.MsgEditStorageProvider
		expectErr bool
		errIs     error
	}{
		{
			name: "fail - set SealAddress to another SP's address",
			req: &sptypes.MsgEditStorageProvider{
				SpAddress:   spA.OperatorAddress,
				SealAddress: spB.SealAddress,
			},
			expectErr: true,
			errIs:     sptypes.ErrStorageProviderSealAddrExists,
		},
		{
			name: "fail - set ApprovalAddress to another SP's address",
			req: &sptypes.MsgEditStorageProvider{
				SpAddress:       spA.OperatorAddress,
				ApprovalAddress: spB.ApprovalAddress,
			},
			expectErr: true,
			errIs:     sptypes.ErrStorageProviderApprovalAddrExists,
		},
		{
			name: "fail - set GcAddress to another SP's address",
			req: &sptypes.MsgEditStorageProvider{
				SpAddress: spA.OperatorAddress,
				GcAddress: spB.GcAddress,
			},
			expectErr: true,
			errIs:     sptypes.ErrStorageProviderGcAddrExists,
		},
		{
			name: "fail - set BlsKey to another SP's key",
			req: &sptypes.MsgEditStorageProvider{
				SpAddress: spA.OperatorAddress,
				BlsKey:    hex.EncodeToString(spB.BlsKey),
				BlsProof:  "00", // won't be verified due to early uniqueness check
			},
			expectErr: true,
			errIs:     sptypes.ErrStorageProviderBlsKeyExists,
		},
		{
			name: "fail - no fields changed",
			req: &sptypes.MsgEditStorageProvider{
				SpAddress: spA.OperatorAddress,
			},
			expectErr: true,
			errIs:     sptypes.ErrStorageProviderNotChanged,
		},
		{
			name: "success - idempotent set SealAddress to current",
			req: &sptypes.MsgEditStorageProvider{
				SpAddress:   spA.OperatorAddress,
				SealAddress: spA.SealAddress,
			},
			expectErr: false,
		},
		{
			name: "success - set SealAddress to a new unique address",
			req: &sptypes.MsgEditStorageProvider{
				SpAddress:   spA.OperatorAddress,
				SealAddress: newSeal.String(),
			},
			expectErr: false,
		},
		{
			name: "success - set BlsKey to a new unique key",
			req: &sptypes.MsgEditStorageProvider{
				SpAddress: spA.OperatorAddress,
				BlsKey:    newBlsPubHex,
				BlsProof:  newBlsProofHex,
			},
			expectErr: false,
		},
	}

	for _, tc := range tests {
		s.Suite.T().Run(tc.name, func(t *testing.T) {
			_, err := s.msgServer.EditStorageProvider(s.ctx, tc.req)
			if tc.expectErr {
				require.Error(t, err)
				if tc.errIs != nil {
					require.ErrorIs(t, err, tc.errIs)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestEditStorageProvider_OldIndexCleanup() {
	// Create SP-A
	spA := s.createTestSP(sample.RandAccAddress())
	oldSeal := spA.SealAddress

	// Edit SP-A to set a new unique SealAddress
	newSeal := sample.RandAccAddress()
	_, err := s.msgServer.EditStorageProvider(s.ctx, &sptypes.MsgEditStorageProvider{
		SpAddress:   spA.OperatorAddress,
		SealAddress: newSeal.String(),
	})
	s.Require().NoError(err)

	// Old index should be removed
	_, found := s.spKeeper.GetStorageProviderBySealAddr(s.ctx, sdk.MustAccAddressFromHex(oldSeal))
	s.Require().False(found)

	// Now create SP-B using the oldSeal; it should succeed (address is released)
	opB := sample.RandAccAddress()
	fundingB := sample.RandAccAddress()
	approvalB := sample.RandAccAddress()
	gcB := sample.RandAccAddress()
	maintenanceB := sample.RandAccAddress()

	// mocks for SP-B
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), fundingB).Return(authtypes.NewBaseAccountWithAddress(fundingB)).AnyTimes()
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	blsHex, blsProof, err := newTestBlsKeyAndProof()
	s.Require().NoError(err)

	_, err = s.msgServer.CreateStorageProvider(s.ctx, &sptypes.MsgCreateStorageProvider{
		Creator:            opB.String(),
		SpAddress:          opB.String(),
		FundingAddress:     fundingB.String(),
		SealAddress:        oldSeal, // reuse oldSeal
		ApprovalAddress:    approvalB.String(),
		GcAddress:          gcB.String(),
		MaintenanceAddress: maintenanceB.String(),
		BlsKey:             blsHex,
		BlsProof:           blsProof,
		Endpoint:           "https://sp2.example",
		Deposit: sdk.Coin{
			Denom:  types.Denom,
			Amount: types.NewIntFromInt64WithDecimal(10000, types.DecimalMOCA),
		},
	})
	s.Require().NoError(err)
}

// putSP stores a storage provider directly in the KV store, bypassing CreateStorageProvider
// (and the account/bank mocks it needs), and wires the same by-address indices it would, so
// tests can target one specific msg-server check without unrelated mock setup.
func (s *KeeperTestSuite) putSP(id uint32, status sptypes.Status, operator, funding, seal, approval, gc, maintenance sdk.AccAddress) *sptypes.StorageProvider {
	sp := &sptypes.StorageProvider{
		Id:                 id,
		Status:             status,
		OperatorAddress:    operator.String(),
		FundingAddress:     funding.String(),
		SealAddress:        seal.String(),
		ApprovalAddress:    approval.String(),
		GcAddress:          gc.String(),
		MaintenanceAddress: maintenance.String(),
		TotalDeposit:       math.ZeroInt(),
	}
	s.spKeeper.SetStorageProvider(s.ctx, sp)
	s.spKeeper.SetStorageProviderByOperatorAddr(s.ctx, sp)
	s.spKeeper.SetStorageProviderByFundingAddr(s.ctx, sp)
	s.spKeeper.SetStorageProviderBySealAddr(s.ctx, sp)
	s.spKeeper.SetStorageProviderByApprovalAddr(s.ctx, sp)
	s.spKeeper.SetStorageProviderByGcAddr(s.ctx, sp)
	return sp
}

// newValidCreateMsg builds a MsgCreateStorageProvider with fresh, unique addresses and a real
// BLS key/proof pair that together pass every check in CreateStorageProvider, and mocks the
// funding-account lookup the handler always performs. Callers mutate individual fields to
// target one specific later check.
func (s *KeeperTestSuite) newValidCreateMsg() *sptypes.MsgCreateStorageProvider {
	opAddr := sample.RandAccAddress()
	fundingAddr := sample.RandAccAddress()
	sealAddr := sample.RandAccAddress()
	approvalAddr := sample.RandAccAddress()
	gcAddr := sample.RandAccAddress()
	maintenanceAddr := sample.RandAccAddress()

	blsPubHex, blsProofHex, err := newTestBlsKeyAndProof()
	s.Require().NoError(err)

	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), fundingAddr).Return(authtypes.NewBaseAccountWithAddress(fundingAddr)).AnyTimes()

	return &sptypes.MsgCreateStorageProvider{
		Creator:            opAddr.String(),
		SpAddress:          opAddr.String(),
		FundingAddress:     fundingAddr.String(),
		SealAddress:        sealAddr.String(),
		ApprovalAddress:    approvalAddr.String(),
		GcAddress:          gcAddr.String(),
		MaintenanceAddress: maintenanceAddr.String(),
		BlsKey:             blsPubHex,
		BlsProof:           blsProofHex,
		Description:        sptypes.Description{Moniker: "msgserver_test_sp"},
		Endpoint:           "https://sp.example",
		Deposit: sdk.Coin{
			Denom:  types.Denom,
			Amount: types.NewIntFromInt64WithDecimal(10000, types.DecimalMOCA),
		},
	}
}

// A signer mismatch is rejected on both sides of the genesis/post-genesis split: before
// genesis the sp operator itself must sign, after genesis only the gov module may.
func (s *KeeperTestSuite) TestCreateStorageProvider_SignerMismatch() {
	s.Suite.T().Run("genesis: signer must be the sp operator", func(t *testing.T) {
		msg := s.newValidCreateMsg()
		msg.Creator = sample.RandAccAddressHex()
		_, err := s.msgServer.CreateStorageProvider(s.ctx, msg)
		require.ErrorIs(t, err, sptypes.ErrSignerNotSPOperator)
	})

	s.Suite.T().Run("post-genesis: signer must be the gov module", func(t *testing.T) {
		msg := s.newValidCreateMsg()
		msg.Creator = sample.RandAccAddressHex()
		govAddr := authtypes.NewModuleAddress(gov.ModuleName)
		s.accountKeeper.EXPECT().GetModuleAddress(gov.ModuleName).Return(govAddr).AnyTimes()

		ctx := s.ctx.WithBlockHeight(1)
		_, err := s.msgServer.CreateStorageProvider(ctx, msg)
		require.ErrorIs(t, err, sptypes.ErrSignerNotGovModule)
	})
}

// Every one of the 5 by-address uniqueness checks, plus the BLS key uniqueness check, must
// reject a new storage provider that reuses a value already registered to another one.
func (s *KeeperTestSuite) TestCreateStorageProvider_DuplicateFields() {
	spA := s.createTestSP(sample.RandAccAddress())

	tests := []struct {
		name    string
		mutate  func(msg *sptypes.MsgCreateStorageProvider)
		wantErr error
	}{
		{
			name: "operator address exists",
			mutate: func(m *sptypes.MsgCreateStorageProvider) {
				m.Creator = spA.OperatorAddress
				m.SpAddress = spA.OperatorAddress
			},
			wantErr: sptypes.ErrStorageProviderOwnerExists,
		},
		{
			name:    "funding address exists",
			mutate:  func(m *sptypes.MsgCreateStorageProvider) { m.FundingAddress = spA.FundingAddress },
			wantErr: sptypes.ErrStorageProviderFundingAddrExists,
		},
		{
			name:    "seal address exists",
			mutate:  func(m *sptypes.MsgCreateStorageProvider) { m.SealAddress = spA.SealAddress },
			wantErr: sptypes.ErrStorageProviderSealAddrExists,
		},
		{
			name:    "approval address exists",
			mutate:  func(m *sptypes.MsgCreateStorageProvider) { m.ApprovalAddress = spA.ApprovalAddress },
			wantErr: sptypes.ErrStorageProviderApprovalAddrExists,
		},
		{
			name:    "gc address exists",
			mutate:  func(m *sptypes.MsgCreateStorageProvider) { m.GcAddress = spA.GcAddress },
			wantErr: sptypes.ErrStorageProviderGcAddrExists,
		},
		{
			name:    "bls key exists",
			mutate:  func(m *sptypes.MsgCreateStorageProvider) { m.BlsKey = hex.EncodeToString(spA.BlsKey) },
			wantErr: sptypes.ErrStorageProviderBlsKeyExists,
		},
	}

	for _, tc := range tests {
		s.Suite.T().Run(tc.name, func(t *testing.T) {
			msg := s.newValidCreateMsg()
			tc.mutate(msg)
			_, err := s.msgServer.CreateStorageProvider(s.ctx, msg)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// A description exceeding the max moniker length is rejected by Description.EnsureLength.
func (s *KeeperTestSuite) TestCreateStorageProvider_DescriptionTooLong() {
	msg := s.newValidCreateMsg()
	msg.Description.Moniker = strings.Repeat("x", sptypes.MaxMonikerLength+1)

	_, err := s.msgServer.CreateStorageProvider(s.ctx, msg)
	require.ErrorContains(s.T(), err, "invalid moniker length")
}

// A deposit below the minimum, or in the wrong denom, is rejected before any funds move.
func (s *KeeperTestSuite) TestCreateStorageProvider_DepositValidation() {
	tests := []struct {
		name    string
		mutate  func(msg *sptypes.MsgCreateStorageProvider)
		wantErr error
	}{
		{
			name: "deposit below minimum",
			mutate: func(m *sptypes.MsgCreateStorageProvider) {
				m.Deposit = sdk.Coin{Denom: types.Denom, Amount: math.NewInt(1)}
			},
			wantErr: sptypes.ErrInsufficientDepositAmount,
		},
		{
			name: "wrong deposit denom",
			mutate: func(m *sptypes.MsgCreateStorageProvider) {
				m.Deposit = sdk.Coin{Denom: "wrongdenom", Amount: types.NewIntFromInt64WithDecimal(10000, types.DecimalMOCA)}
			},
			wantErr: sptypes.ErrInvalidDenom,
		},
	}

	for _, tc := range tests {
		s.Suite.T().Run(tc.name, func(t *testing.T) {
			msg := s.newValidCreateMsg()
			tc.mutate(msg)
			_, err := s.msgServer.CreateStorageProvider(s.ctx, msg)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// A failure moving the deposit from the funding account to the module account must propagate
// as-is and not be swallowed.
func (s *KeeperTestSuite) TestCreateStorageProvider_BankTransferError() {
	msg := s.newValidCreateMsg()
	bankErr := errors.New("mock bank failure")
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(bankErr)

	_, err := s.msgServer.CreateStorageProvider(s.ctx, msg)
	require.ErrorIs(s.T(), err, bankErr)
}

// Post-genesis, creating a storage provider additionally requires a deposit authorization
// grant from the funding address to the gov module, and the new SP starts in maintenance
// rather than in service.
func (s *KeeperTestSuite) TestCreateStorageProvider_PostGenesisAuthorization() {
	govAddr := authtypes.NewModuleAddress(gov.ModuleName)
	ctx := s.ctx.WithBlockHeight(1)

	s.Suite.T().Run("no authorization granted", func(t *testing.T) {
		msg := s.newValidCreateMsg()
		msg.Creator = govAddr.String()
		fundingAcc := sdk.MustAccAddressFromHex(msg.FundingAddress)

		s.accountKeeper.EXPECT().GetModuleAddress(gov.ModuleName).Return(govAddr).AnyTimes()
		s.authzKeeper.EXPECT().GetGrant(gomock.Any(), govAddr, fundingAcc, gomock.Any()).Return(authz.Grant{}, false)

		_, err := s.msgServer.CreateStorageProvider(ctx, msg)
		require.ErrorIs(t, err, authz.ErrNoAuthorizationFound)
	})

	s.Suite.T().Run("authorization granted, sp starts in maintenance", func(t *testing.T) {
		msg := s.newValidCreateMsg()
		msg.Creator = govAddr.String()
		fundingAcc := sdk.MustAccAddressFromHex(msg.FundingAddress)
		spAcc := sdk.MustAccAddressFromHex(msg.SpAddress)

		s.accountKeeper.EXPECT().GetModuleAddress(gov.ModuleName).Return(govAddr).AnyTimes()
		s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil)

		maxDeposit := msg.Deposit
		grant, err := authz.NewGrant(ctx.BlockTime(), sptypes.NewDepositAuthorization(spAcc, &maxDeposit), nil)
		require.NoError(t, err)
		s.authzKeeper.EXPECT().GetGrant(gomock.Any(), govAddr, fundingAcc, gomock.Any()).Return(grant, true)
		// the grant's MaxDeposit exactly matches the deposit, so it is fully consumed and deleted.
		s.authzKeeper.EXPECT().DeleteGrant(gomock.Any(), govAddr, fundingAcc, gomock.Any()).Return(nil)

		_, err = s.msgServer.CreateStorageProvider(ctx, msg)
		require.NoError(t, err)

		sp, found := s.spKeeper.GetStorageProviderByOperatorAddr(ctx, spAcc)
		require.True(t, found)
		require.Equal(t, sptypes.STATUS_IN_MAINTENANCE, sp.Status)
	})
}

// checkBlsProof rejects a malformed proof (bad hex, wrong length, or well-formed-but-invalid
// bytes), a well-formed-but-invalid public key that bypassed the caller's length-only check,
// and a validly-shaped proof that simply does not verify against the given key. All are
// exercised through CreateStorageProvider, the only exported entry point that reaches it.
func (s *KeeperTestSuite) TestCheckBlsProof_ErrorBranches() {
	validPubHex, validProofHex, err := newTestBlsKeyAndProof()
	s.Require().NoError(err)
	_, otherProofHex, err := newTestBlsKeyAndProof()
	s.Require().NoError(err)

	zeroPubKeyHex := hex.EncodeToString(make([]byte, sdk.BLSPubKeyLength))
	zeroSigHex := hex.EncodeToString(make([]byte, sdk.BLSSignatureLength))

	tests := []struct {
		name     string
		blsKey   string
		blsProof string
	}{
		{"bad hex proof", validPubHex, "not-a-hex-string!!"},
		{"wrong length proof", validPubHex, "aabbcc"},
		{"right-length but invalid proof bytes", validPubHex, zeroSigHex},
		{"right-length but invalid pubkey bytes", zeroPubKeyHex, validProofHex},
		{"pubkey and proof from different keypairs", validPubHex, otherProofHex},
	}
	for _, tc := range tests {
		s.Suite.T().Run(tc.name, func(t *testing.T) {
			msg := s.newValidCreateMsg()
			msg.BlsKey = tc.blsKey
			msg.BlsProof = tc.blsProof
			_, err := s.msgServer.CreateStorageProvider(s.ctx, msg)
			require.Error(t, err)
		})
	}
}

// A malformed maintenance address is rejected before any state is touched, and a malformed
// (or wrong-length) BLS key is rejected by CreateStorageProvider's own format check, before
// ever reaching checkBlsProof.
func (s *KeeperTestSuite) TestCreateStorageProvider_MalformedAddressAndKey() {
	tests := []struct {
		name    string
		mutate  func(msg *sptypes.MsgCreateStorageProvider)
		wantErr error
	}{
		{
			name:   "malformed maintenance address",
			mutate: func(m *sptypes.MsgCreateStorageProvider) { m.MaintenanceAddress = "not-a-valid-hex-address" },
		},
		{
			name:    "malformed bls key",
			mutate:  func(m *sptypes.MsgCreateStorageProvider) { m.BlsKey = "zz" },
			wantErr: sptypes.ErrStorageProviderInvalidBlsKey,
		},
	}
	for _, tc := range tests {
		s.Suite.T().Run(tc.name, func(t *testing.T) {
			msg := s.newValidCreateMsg()
			tc.mutate(msg)
			_, err := s.msgServer.CreateStorageProvider(s.ctx, msg)
			require.Error(t, err)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			}
		})
	}
}

// Editing a storage provider that does not exist is rejected.
func (s *KeeperTestSuite) TestEditStorageProvider_NotFound() {
	_, err := s.msgServer.EditStorageProvider(s.ctx, &sptypes.MsgEditStorageProvider{
		SpAddress: sample.RandAccAddressHex(),
		Endpoint:  "https://new.example",
	})
	require.ErrorIs(s.T(), err, sptypes.ErrStorageProviderNotFound)
}

// A valid edit updates the endpoint and, via Description.UpdateDescription, the description,
// leaving fields marked do-not-modify untouched.
func (s *KeeperTestSuite) TestEditStorageProvider_EndpointAndDescription() {
	spA := s.createTestSP(sample.RandAccAddress())

	_, err := s.msgServer.EditStorageProvider(s.ctx, &sptypes.MsgEditStorageProvider{
		SpAddress: spA.OperatorAddress,
		Endpoint:  "https://updated.example",
		Description: &sptypes.Description{
			Moniker:  "updated moniker",
			Identity: sptypes.DoNotModifyDesc,
			Website:  sptypes.DoNotModifyDesc,
			Details:  sptypes.DoNotModifyDesc,
		},
	})
	s.Require().NoError(err)

	updated, found := s.spKeeper.GetStorageProviderByOperatorAddr(s.ctx, sdk.MustAccAddressFromHex(spA.OperatorAddress))
	s.Require().True(found)
	s.Require().Equal("https://updated.example", updated.Endpoint)
	s.Require().Equal("updated moniker", updated.Description.Moniker)
	s.Require().Equal(spA.Description.Identity, updated.Description.Identity)
}

// A description update that would exceed the max moniker length is rejected, and does not
// change any other field.
func (s *KeeperTestSuite) TestEditStorageProvider_DescriptionTooLong() {
	spA := s.createTestSP(sample.RandAccAddress())

	_, err := s.msgServer.EditStorageProvider(s.ctx, &sptypes.MsgEditStorageProvider{
		SpAddress: spA.OperatorAddress,
		Description: &sptypes.Description{
			Moniker:  strings.Repeat("x", sptypes.MaxMonikerLength+1),
			Identity: sptypes.DoNotModifyDesc,
			Website:  sptypes.DoNotModifyDesc,
			Details:  sptypes.DoNotModifyDesc,
		},
	})
	require.ErrorContains(s.T(), err, "invalid moniker length")
}

// Changing ApprovalAddress and GcAddress must delete their old index entries (freeing them
// for reuse) and persist the new ones, mirroring the seal/bls coverage in
// TestEditStorageProvider_OldIndexCleanup.
func (s *KeeperTestSuite) TestEditStorageProvider_ApprovalAndGcIndexCleanup() {
	spA := s.createTestSP(sample.RandAccAddress())
	oldApproval := spA.ApprovalAddress
	oldGc := spA.GcAddress

	newApproval := sample.RandAccAddress()
	newGc := sample.RandAccAddress()

	_, err := s.msgServer.EditStorageProvider(s.ctx, &sptypes.MsgEditStorageProvider{
		SpAddress:       spA.OperatorAddress,
		ApprovalAddress: newApproval.String(),
		GcAddress:       newGc.String(),
	})
	s.Require().NoError(err)

	_, found := s.spKeeper.GetStorageProviderByApprovalAddr(s.ctx, sdk.MustAccAddressFromHex(oldApproval))
	s.Require().False(found, "old approval address index must be removed")
	_, found = s.spKeeper.GetStorageProviderByGcAddr(s.ctx, sdk.MustAccAddressFromHex(oldGc))
	s.Require().False(found, "old gc address index must be removed")

	updated, found := s.spKeeper.GetStorageProviderByOperatorAddr(s.ctx, sdk.MustAccAddressFromHex(spA.OperatorAddress))
	s.Require().True(found)
	s.Require().Equal(newApproval.String(), updated.ApprovalAddress)
	s.Require().Equal(newGc.String(), updated.GcAddress)
}

// MaintenanceAddress has no uniqueness index, so editing it is a direct field replacement.
func (s *KeeperTestSuite) TestEditStorageProvider_MaintenanceAddress() {
	spA := s.createTestSP(sample.RandAccAddress())
	newMaintenance := sample.RandAccAddress()

	_, err := s.msgServer.EditStorageProvider(s.ctx, &sptypes.MsgEditStorageProvider{
		SpAddress:          spA.OperatorAddress,
		MaintenanceAddress: newMaintenance.String(),
	})
	s.Require().NoError(err)

	updated, found := s.spKeeper.GetStorageProviderByOperatorAddr(s.ctx, sdk.MustAccAddressFromHex(spA.OperatorAddress))
	s.Require().True(found)
	s.Require().Equal(newMaintenance.String(), updated.MaintenanceAddress)
}

// EditStorageProvider validates a replacement BLS key the same way CreateStorageProvider
// does: an invalid key format is rejected outright, and a key/proof pair that fails
// checkBlsProof propagates that failure.
func (s *KeeperTestSuite) TestEditStorageProvider_BlsErrors() {
	spA := s.createTestSP(sample.RandAccAddress())
	freshPubHex, _, err := newTestBlsKeyAndProof()
	s.Require().NoError(err)

	tests := []struct {
		name     string
		blsKey   string
		blsProof string
		wantErr  error
	}{
		{"invalid bls key format", "zz", "aabbcc", sptypes.ErrStorageProviderInvalidBlsKey},
		{"checkBlsProof failure", freshPubHex, "aabbcc", nil},
	}
	for _, tc := range tests {
		s.Suite.T().Run(tc.name, func(t *testing.T) {
			_, err := s.msgServer.EditStorageProvider(s.ctx, &sptypes.MsgEditStorageProvider{
				SpAddress: spA.OperatorAddress,
				BlsKey:    tc.blsKey,
				BlsProof:  tc.blsProof,
			})
			require.Error(t, err)
			if tc.wantErr != nil {
				require.ErrorIs(t, err, tc.wantErr)
			}
		})
	}
}

// Deposit rejects an unregistered funding address, a funding address bound to a different
// sp, the wrong denom, and a failed bank transfer.
func (s *KeeperTestSuite) TestMsgDeposit_Errors() {
	opAddr := sample.RandAccAddress()
	fundAddr := sample.RandAccAddress()
	otherAddr := sample.RandAccAddress()
	sp := s.putSP(1, sptypes.STATUS_IN_SERVICE, opAddr, fundAddr, sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress())

	bankErr := errors.New("mock bank failure")
	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(bankErr).AnyTimes()

	validAmt := types.NewIntFromInt64WithDecimal(1, types.DecimalMOCA)

	tests := []struct {
		name    string
		req     *sptypes.MsgDeposit
		wantErr error
	}{
		{
			name:    "funding address not registered",
			req:     &sptypes.MsgDeposit{Creator: otherAddr.String(), SpAddress: sp.OperatorAddress, Deposit: sdk.Coin{Denom: types.Denom, Amount: validAmt}},
			wantErr: sptypes.ErrStorageProviderNotFound,
		},
		{
			name:    "sp address mismatch",
			req:     &sptypes.MsgDeposit{Creator: fundAddr.String(), SpAddress: otherAddr.String(), Deposit: sdk.Coin{Denom: types.Denom, Amount: validAmt}},
			wantErr: sptypes.ErrDepositAccountNotAllowed,
		},
		{
			name:    "wrong denom",
			req:     &sptypes.MsgDeposit{Creator: fundAddr.String(), SpAddress: sp.OperatorAddress, Deposit: sdk.Coin{Denom: "wrongdenom", Amount: validAmt}},
			wantErr: sptypes.ErrInvalidDenom,
		},
		{
			name:    "bank transfer fails",
			req:     &sptypes.MsgDeposit{Creator: fundAddr.String(), SpAddress: sp.OperatorAddress, Deposit: sdk.Coin{Denom: types.Denom, Amount: validAmt}},
			wantErr: bankErr,
		},
	}
	for _, tc := range tests {
		s.Suite.T().Run(tc.name, func(t *testing.T) {
			_, err := s.msgServer.Deposit(s.ctx, tc.req)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// A successful deposit increases TotalDeposit by exactly the deposited amount and emits
// EventDeposit.
func (s *KeeperTestSuite) TestMsgDeposit_Success() {
	opAddr := sample.RandAccAddress()
	fundAddr := sample.RandAccAddress()
	sp := s.putSP(1, sptypes.STATUS_IN_SERVICE, opAddr, fundAddr, sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress())

	s.bankKeeper.EXPECT().SendCoinsFromAccountToModule(gomock.Any(), fundAddr, sptypes.ModuleName, gomock.Any()).Return(nil)

	depositAmt := types.NewIntFromInt64WithDecimal(500, types.DecimalMOCA)
	_, err := s.msgServer.Deposit(s.ctx, &sptypes.MsgDeposit{
		Creator:   fundAddr.String(),
		SpAddress: sp.OperatorAddress,
		Deposit:   sdk.Coin{Denom: types.Denom, Amount: depositAmt},
	})
	s.Require().NoError(err)

	got, found := s.spKeeper.GetStorageProvider(s.ctx, sp.Id)
	s.Require().True(found)
	s.Require().True(got.TotalDeposit.Equal(depositAmt))

	var eventFound bool
	for _, ev := range s.ctx.EventManager().Events() {
		if ev.Type == proto.MessageName(&sptypes.EventDeposit{}) {
			eventFound = true
		}
	}
	s.Require().True(eventFound, "expected EventDeposit to be emitted")
}

// UpdateSpStoragePrice rejects an unknown sp, an sp that is not in service, and any update
// requested in the last UpdatePriceDisallowedDays of the month; otherwise it stores the new
// price.
func (s *KeeperTestSuite) TestMsgUpdateSpStoragePrice() {
	sp := s.putSP(1, sptypes.STATUS_IN_SERVICE, sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress())
	maintSp := s.putSP(2, sptypes.STATUS_IN_MAINTENANCE, sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress())
	notFoundAddr := sample.RandAccAddress()
	validPrice := math.LegacyNewDec(100)

	// 2023-08-30 UTC falls in the last 2 (default) days of August; 2023-08-28 UTC does not.
	disallowedTime := time.Unix(1693414861, 0)
	allowedTime := time.Unix(1693242061, 0)

	tests := []struct {
		name    string
		ctx     sdk.Context
		req     *sptypes.MsgUpdateSpStoragePrice
		wantErr error
	}{
		{
			name:    "not found",
			ctx:     s.ctx.WithBlockTime(allowedTime),
			req:     &sptypes.MsgUpdateSpStoragePrice{SpAddress: notFoundAddr.String(), ReadPrice: validPrice, StorePrice: validPrice},
			wantErr: sptypes.ErrStorageProviderNotFound,
		},
		{
			name:    "not in service",
			ctx:     s.ctx.WithBlockTime(allowedTime),
			req:     &sptypes.MsgUpdateSpStoragePrice{SpAddress: maintSp.OperatorAddress, ReadPrice: validPrice, StorePrice: validPrice},
			wantErr: sptypes.ErrStorageProviderNotInService,
		},
		{
			name:    "disallowed in the last days of the month",
			ctx:     s.ctx.WithBlockTime(disallowedTime),
			req:     &sptypes.MsgUpdateSpStoragePrice{SpAddress: sp.OperatorAddress, ReadPrice: validPrice, StorePrice: validPrice},
			wantErr: sptypes.ErrStorageProviderPriceUpdateNotAllow,
		},
	}
	for _, tc := range tests {
		s.Suite.T().Run(tc.name, func(t *testing.T) {
			_, err := s.msgServer.UpdateSpStoragePrice(tc.ctx, tc.req)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}

	happyCtx := s.ctx.WithBlockTime(allowedTime)
	_, err := s.msgServer.UpdateSpStoragePrice(happyCtx, &sptypes.MsgUpdateSpStoragePrice{
		SpAddress:     sp.OperatorAddress,
		ReadPrice:     validPrice,
		StorePrice:    validPrice,
		FreeReadQuota: 100,
	})
	s.Require().NoError(err)

	price, found := s.spKeeper.GetSpStoragePrice(happyCtx, sp.Id)
	s.Require().True(found)
	s.Require().True(price.ReadPrice.Equal(validPrice))
	s.Require().True(price.StorePrice.Equal(validPrice))
	s.Require().EqualValues(100, price.FreeReadQuota)
}

// UpdateParams rejects a request from anyone but the module's own authority, and rejects a
// syntactically-built but invalid new parameter set via SetParams' own validation; a valid
// update from the real authority is persisted and emits a Params event.
func (s *KeeperTestSuite) TestMsgUpdateParams() {
	authority := s.spKeeper.GetAuthority()

	s.Suite.T().Run("wrong authority", func(t *testing.T) {
		_, err := s.msgServer.UpdateParams(s.ctx, &sptypes.MsgUpdateParams{
			Authority: sample.RandAccAddressHex(),
			Params:    sptypes.DefaultParams(),
		})
		require.ErrorIs(t, err, gov.ErrInvalidSigner)
	})

	s.Suite.T().Run("invalid new params", func(t *testing.T) {
		_, err := s.msgServer.UpdateParams(s.ctx, &sptypes.MsgUpdateParams{
			Authority: authority,
			Params:    sptypes.Params{},
		})
		require.ErrorContains(t, err, "deposit denom cannot be blank")
	})

	newParams := sptypes.DefaultParams()
	newParams.MaintenanceDurationQuota = sptypes.DefaultMaintenanceDurationQuota + 100
	_, err := s.msgServer.UpdateParams(s.ctx, &sptypes.MsgUpdateParams{Authority: authority, Params: newParams})
	s.Require().NoError(err)
	s.Require().Equal(newParams, s.spKeeper.GetParams(s.ctx))

	var eventFound bool
	for _, ev := range s.ctx.EventManager().Events() {
		if ev.Type == proto.MessageName(&newParams) {
			eventFound = true
		}
	}
	s.Require().True(eventFound, "expected a Params event to be emitted")
}

// UpdateSpStatus rejects an unknown sp, a no-op status request, any in-service-to-non-
// maintenance or maintenance-to-non-service transition, an in-service-to-maintenance
// transition that exceeds the maintenance quota, and any transition requested while jailed
// or exiting.
func (s *KeeperTestSuite) TestMsgUpdateSpStatus_Errors() {
	notFoundAddr := sample.RandAccAddress()

	mk := func(id uint32, status sptypes.Status) *sptypes.StorageProvider {
		return s.putSP(id, status, sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress())
	}

	sameStatus := mk(1, sptypes.STATUS_IN_SERVICE)
	inService := mk(2, sptypes.STATUS_IN_SERVICE)
	quotaExceeded := mk(3, sptypes.STATUS_IN_SERVICE)
	inMaintenance := mk(4, sptypes.STATUS_IN_MAINTENANCE)
	jailed := mk(5, sptypes.STATUS_IN_JAILED)
	gracefulExiting := mk(6, sptypes.STATUS_GRACEFUL_EXITING)
	forcedExiting := mk(7, sptypes.STATUS_FORCED_EXITING)

	tests := []struct {
		name    string
		req     *sptypes.MsgUpdateStorageProviderStatus
		wantErr error
	}{
		{"not found", &sptypes.MsgUpdateStorageProviderStatus{SpAddress: notFoundAddr.String(), Status: sptypes.STATUS_IN_MAINTENANCE, Duration: 10}, sptypes.ErrStorageProviderNotFound},
		{"no-op, same status", &sptypes.MsgUpdateStorageProviderStatus{SpAddress: sameStatus.OperatorAddress, Status: sptypes.STATUS_IN_SERVICE}, sptypes.ErrStorageProviderNotChanged},
		{"in-service to jailed rejected", &sptypes.MsgUpdateStorageProviderStatus{SpAddress: inService.OperatorAddress, Status: sptypes.STATUS_IN_JAILED}, sptypes.ErrStorageProviderStatusUpdateNotAllow},
		{
			"in-service to maintenance over quota",
			&sptypes.MsgUpdateStorageProviderStatus{SpAddress: quotaExceeded.OperatorAddress, Status: sptypes.STATUS_IN_MAINTENANCE, Duration: sptypes.DefaultMaintenanceDurationQuota + 1},
			sptypes.ErrStorageProviderStatusUpdateNotAllow,
		},
		{"in-maintenance to jailed rejected", &sptypes.MsgUpdateStorageProviderStatus{SpAddress: inMaintenance.OperatorAddress, Status: sptypes.STATUS_IN_JAILED}, sptypes.ErrStorageProviderStatusUpdateNotAllow},
		{"jailed cannot self-update", &sptypes.MsgUpdateStorageProviderStatus{SpAddress: jailed.OperatorAddress, Status: sptypes.STATUS_IN_SERVICE}, sptypes.ErrStorageProviderStatusUpdateNotAllow},
		{"graceful exiting cannot self-update", &sptypes.MsgUpdateStorageProviderStatus{SpAddress: gracefulExiting.OperatorAddress, Status: sptypes.STATUS_IN_SERVICE}, sptypes.ErrStorageProviderStatusUpdateNotAllow},
		{"forced exiting cannot self-update", &sptypes.MsgUpdateStorageProviderStatus{SpAddress: forcedExiting.OperatorAddress, Status: sptypes.STATUS_IN_SERVICE}, sptypes.ErrStorageProviderStatusUpdateNotAllow},
	}
	for _, tc := range tests {
		s.Suite.T().Run(tc.name, func(t *testing.T) {
			_, err := s.msgServer.UpdateSpStatus(s.ctx, tc.req)
			require.ErrorIs(t, err, tc.wantErr)
		})
	}
}

// A valid in-service-to-maintenance transition stores the new status and emits
// EventUpdateStorageProviderStatus.
func (s *KeeperTestSuite) TestMsgUpdateSpStatus_ToMaintenance_Success() {
	sp := s.putSP(1, sptypes.STATUS_IN_SERVICE, sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress())

	_, err := s.msgServer.UpdateSpStatus(s.ctx, &sptypes.MsgUpdateStorageProviderStatus{
		SpAddress: sp.OperatorAddress,
		Status:    sptypes.STATUS_IN_MAINTENANCE,
		Duration:  100,
	})
	s.Require().NoError(err)

	got, found := s.spKeeper.GetStorageProvider(s.ctx, sp.Id)
	s.Require().True(found)
	s.Require().Equal(sptypes.STATUS_IN_MAINTENANCE, got.Status)

	var eventFound bool
	for _, ev := range s.ctx.EventManager().Events() {
		if ev.Type == proto.MessageName(&sptypes.EventUpdateStorageProviderStatus{}) {
			eventFound = true
		}
	}
	s.Require().True(eventFound, "expected EventUpdateStorageProviderStatus to be emitted")
}

// A valid in-maintenance-to-in-service transition stores the new status.
func (s *KeeperTestSuite) TestMsgUpdateSpStatus_ToInService_Success() {
	sp := s.putSP(1, sptypes.STATUS_IN_MAINTENANCE, sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress(), sample.RandAccAddress())

	_, err := s.msgServer.UpdateSpStatus(s.ctx, &sptypes.MsgUpdateStorageProviderStatus{
		SpAddress: sp.OperatorAddress,
		Status:    sptypes.STATUS_IN_SERVICE,
	})
	s.Require().NoError(err)

	got, found := s.spKeeper.GetStorageProvider(s.ctx, sp.Id)
	s.Require().True(found)
	s.Require().Equal(sptypes.STATUS_IN_SERVICE, got.Status)
}
