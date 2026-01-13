package keeper_test

import (
	"fmt"
	"time"

	sdk "github.com/cosmos/cosmos-sdk/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/mock/gomock"

	"github.com/evmos/evmos/v12/testutil/sample"
	"github.com/evmos/evmos/v12/types/common"
	sptypes "github.com/evmos/evmos/v12/x/sp/types"
	types "github.com/evmos/evmos/v12/x/storage/types"
	vgtypes "github.com/evmos/evmos/v12/x/virtualgroup/types"
)

// Compatibility regression tests: only rely on public behavior that existed before the fix

// LOW-015: set max buckets per account to 1; create twice with the same owner; the second must fail
// Before fix: erroneously succeeds → this test fails (proves the bug)
// After fix: correctly fails → this test passes (proves the fix)
func (s *TestSuite) TestCompat_LOW015_CreateOverLimit_Fails() {
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{Id: 1, PrimarySpId: sp.Id}

	// Minimal mocks required for the public path only
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).Return(gvgFamily, nil).AnyTimes()

	// Set limit to 1
	params := s.storageKeeper.GetParams(s.ctx)
	params.MaxBucketsPerAccount = 1
	s.Require().NoError(s.storageKeeper.SetParams(s.ctx, params))

	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)

	// First creation: should succeed (both before and after fix)
	bucketName1 := fmt.Sprintf("compat-low15-bucket-1-%d", time.Now().UnixNano())
	// Generate a test approval key and align SP.ApprovalAddress with it
	privKey, err := gethcrypto.GenerateKey()
	s.Require().NoError(err)
	sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex()
	// Build approval bytes using MsgCreateBucket.GetApprovalBytes()
	msg1 := &types.MsgCreateBucket{
		Creator:          owner.String(),
		BucketName:       bucketName1,
		Visibility:       types.VISIBILITY_TYPE_PRIVATE,
		PaymentAddress:   owner.String(),
		PrimarySpAddress: primarySpAddr.String(),
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              uint64(s.ctx.BlockHeight() + 1000),
			GlobalVirtualGroupFamilyId: gvgFamily.Id,
			Sig:                        nil,
		},
		ChargedReadQuota: 0,
	}
	approvalBytes1 := msg1.GetApprovalBytes()
	hash1 := gethcrypto.Keccak256(approvalBytes1)
	sig1, err := gethcrypto.Sign(hash1, privKey)
	s.Require().NoError(err)

	opts := &types.CreateBucketOptions{
		Visibility:       types.VISIBILITY_TYPE_PRIVATE,
		SourceType:       types.SOURCE_TYPE_ORIGIN,
		ChargedReadQuota: 0,
		PaymentAddress:   owner.String(),
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              msg1.PrimarySpApproval.ExpiredHeight,
			GlobalVirtualGroupFamilyId: gvgFamily.Id,
			Sig:                        sig1,
		},
		ApprovalMsgBytes: approvalBytes1,
	}
	_, err = s.storageKeeper.CreateBucket(s.ctx, owner, bucketName1, primarySpAddr, opts)
	s.Require().NoError(err)

	// Second creation: should fail after fix; will erroneously succeed before fix (so this test fails)
	bucketName2 := fmt.Sprintf("compat-low15-bucket-2-%d", time.Now().UnixNano())
	// update message for new bucket name
	msg2 := *msg1
	msg2.BucketName = bucketName2
	approvalBytes2 := msg2.GetApprovalBytes()
	hash2 := gethcrypto.Keccak256(approvalBytes2)
	sig2, err := gethcrypto.Sign(hash2, privKey)
	s.Require().NoError(err)
	opts2 := *opts
	opts2.ApprovalMsgBytes = approvalBytes2
	opts2.PrimarySpApproval = &common.Approval{
		ExpiredHeight:              msg2.PrimarySpApproval.ExpiredHeight,
		GlobalVirtualGroupFamilyId: gvgFamily.Id,
		Sig:                        sig2,
	}
	_, err = s.storageKeeper.CreateBucket(s.ctx, owner, bucketName2, primarySpAddr, &opts2)
	s.Require().Error(err)
}

// INFO-019: expired or invalid signature must fail (either case reveals the issue)
// Before fix: erroneously succeeds → this test fails; after fix: fails → this test passes
func (s *TestSuite) TestCompat_INFO019_ApprovalExpiredOrInvalidSig_Fails() {
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{Id: 1, PrimarySpId: sp.Id}

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).Return(gvgFamily, nil).AnyTimes()

	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)

	// Ensure block height is sufficiently large to test expiration logic
	currentHeight := s.ctx.BlockHeight()
	if currentHeight < 10 {
		s.ctx = s.ctx.WithBlockHeight(100)
	}

	// Case 1: expired approval should fail (before fix it erroneously succeeds)
	bucketExpired := fmt.Sprintf("compat-info19-expired-%d", time.Now().UnixNano())
	// Prepare key/address for approval signer and align SP
	privKey, err := gethcrypto.GenerateKey()
	s.Require().NoError(err)
	sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex()
	// Build expired approval bytes via message helper
	expiredMsg := &types.MsgCreateBucket{
		Creator:          owner.String(),
		BucketName:       bucketExpired,
		Visibility:       types.VISIBILITY_TYPE_PRIVATE,
		PaymentAddress:   owner.String(),
		PrimarySpAddress: primarySpAddr.String(),
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              uint64(s.ctx.BlockHeight() - 10), // clearly expired
			GlobalVirtualGroupFamilyId: gvgFamily.Id,
			Sig:                        nil,
		},
		ChargedReadQuota: 0,
	}
	expiredBytes := expiredMsg.GetApprovalBytes()
	hashExpired := gethcrypto.Keccak256(expiredBytes)
	expiredSig, err := gethcrypto.Sign(hashExpired, privKey)
	s.Require().NoError(err)

	expiredOpts := &types.CreateBucketOptions{
		Visibility:       types.VISIBILITY_TYPE_PRIVATE,
		SourceType:       types.SOURCE_TYPE_ORIGIN,
		ChargedReadQuota: 0,
		PaymentAddress:   owner.String(),
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              expiredMsg.PrimarySpApproval.ExpiredHeight, // expired
			GlobalVirtualGroupFamilyId: gvgFamily.Id,
			Sig:                        expiredSig, // signature exists, but approval is expired → must fail
		},
		ApprovalMsgBytes: expiredBytes,
	}
	_, err = s.storageKeeper.CreateBucket(s.ctx, owner, bucketExpired, primarySpAddr, expiredOpts)
	s.Require().Error(err)

	// Case 2: invalid signature should fail (before fix it erroneously succeeds)
	bucketInvalid := fmt.Sprintf("compat-info19-invalidsig-%d", time.Now().UnixNano())
	// Build non-expired approval bytes and then tamper signature
	validMsg := &types.MsgCreateBucket{
		Creator:          owner.String(),
		BucketName:       bucketInvalid,
		Visibility:       types.VISIBILITY_TYPE_PRIVATE,
		PaymentAddress:   owner.String(),
		PrimarySpAddress: primarySpAddr.String(),
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              uint64(s.ctx.BlockHeight() + 1000), // not expired
			GlobalVirtualGroupFamilyId: gvgFamily.Id,
			Sig:                        nil,
		},
		ChargedReadQuota: 0,
	}
	validBytes := validMsg.GetApprovalBytes()
	// Sign with a different key to make signature invalid for sp.ApprovalAddress
	otherKey, err := gethcrypto.GenerateKey()
	s.Require().NoError(err)
	invalidHash := gethcrypto.Keccak256(validBytes)
	invalidSig, err := gethcrypto.Sign(invalidHash, otherKey)
	s.Require().NoError(err)

	invalidSigOpts := &types.CreateBucketOptions{
		Visibility:       types.VISIBILITY_TYPE_PRIVATE,
		SourceType:       types.SOURCE_TYPE_ORIGIN,
		ChargedReadQuota: 0,
		PaymentAddress:   owner.String(),
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              validMsg.PrimarySpApproval.ExpiredHeight, // not expired
			GlobalVirtualGroupFamilyId: gvgFamily.Id,
			Sig:                        invalidSig, // invalid signature (signed by other key)
		},
		ApprovalMsgBytes: validBytes, // fixed version verifies signature and fails
	}
	_, err = s.storageKeeper.CreateBucket(s.ctx, owner, bucketInvalid, primarySpAddr, invalidSigOpts)
	s.Require().Error(err)
}

// Additional negative tests for new strict checks
func (s *TestSuite) TestCreateBucket_NilPrimarySpApproval_Fails() {
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{Id: 1, PrimarySpId: sp.Id}

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).Return(gvgFamily, nil).AnyTimes()

	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)
	bucketName := fmt.Sprintf("nil-approval-%d", time.Now().UnixNano())

	opts := &types.CreateBucketOptions{
		Visibility:        types.VISIBILITY_TYPE_PRIVATE,
		SourceType:        types.SOURCE_TYPE_ORIGIN,
		ChargedReadQuota:  0,
		PaymentAddress:    owner.String(),
		PrimarySpApproval: nil,                   // nil approval → must fail
		ApprovalMsgBytes:  []byte("placeholder"), // won't be used
	}
	_, err := s.storageKeeper.CreateBucket(s.ctx, owner, bucketName, primarySpAddr, opts)
	s.Require().Error(err)
}

func (s *TestSuite) TestCreateBucket_MissingApprovalMsgBytes_Fails() {
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{Id: 1, PrimarySpId: sp.Id}

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).Return(gvgFamily, nil).AnyTimes()

	// Prepare key/address for approval signer and align SP
	privKey, err := gethcrypto.GenerateKey()
	s.Require().NoError(err)
	sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex()

	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)
	bucketName := fmt.Sprintf("missing-approval-bytes-%d", time.Now().UnixNano())

	opts := &types.CreateBucketOptions{
		Visibility:       types.VISIBILITY_TYPE_PRIVATE,
		SourceType:       types.SOURCE_TYPE_ORIGIN,
		ChargedReadQuota: 0,
		PaymentAddress:   owner.String(),
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              uint64(s.ctx.BlockHeight() + 1000),
			GlobalVirtualGroupFamilyId: gvgFamily.Id,
			Sig:                        []byte("any"), // will be ignored since bytes are missing
		},
		ApprovalMsgBytes: nil, // missing → must fail
	}
	_, err = s.storageKeeper.CreateBucket(s.ctx, owner, bucketName, primarySpAddr, opts)
	s.Require().Error(err)
}
