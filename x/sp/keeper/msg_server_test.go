package keeper_test

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/cometbft/cometbft/crypto/tmhash"
	"github.com/cometbft/cometbft/votepool"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	gov "github.com/cosmos/cosmos-sdk/x/gov/types"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/evmos/evmos/v12/sdk/types"
	"github.com/evmos/evmos/v12/testutil/sample"
	"github.com/evmos/evmos/v12/x/sp/keeper"
	sptypes "github.com/evmos/evmos/v12/x/sp/types"
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
