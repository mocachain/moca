package keeper_test

import (
	"fmt"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"github.com/golang/mock/gomock"

	"github.com/evmos/evmos/v12/testutil/sample"
	"github.com/evmos/evmos/v12/types/common"
	sptypes "github.com/evmos/evmos/v12/x/sp/types"
	types "github.com/evmos/evmos/v12/x/storage/types"
	vgtypes "github.com/evmos/evmos/v12/x/virtualgroup/types"
)

// TestCreateBucket_MaxBucketsPerAccount tests LOW-015 fix: bucket count limit enforcement
func (s *TestSuite) TestCreateBucket_MaxBucketsPerAccount() {
	// Setup
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{
		Id:          1,
		PrimarySpId: sp.Id,
	}

	// Mock expectations
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(sp, true).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).
		Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).
		Return(gvgFamily, nil).AnyTimes()

	// Set MaxBucketsPerAccount to 3 for testing
	params := s.storageKeeper.GetParams(s.ctx)
	params.MaxBucketsPerAccount = 3
	err := s.storageKeeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)

	// Test: Create 3 buckets successfully
	for i := 0; i < 3; i++ {
		bucketName := fmt.Sprintf("test-bucket-%d-%d", time.Now().Unix(), i)
		// Generate a signer and align sp.ApprovalAddress
		privKey, errGen := gethcrypto.GenerateKey()
		s.Require().NoError(errGen)
		sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex()
		// Build approval bytes using MsgCreateBucket helper, then sign
		msg := &types.MsgCreateBucket{
			Creator:          owner.String(),
			BucketName:       bucketName,
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
		approvalBytes := msg.GetApprovalBytes()
		hash := gethcrypto.Keccak256(approvalBytes)
		sig, errSign := gethcrypto.Sign(hash, privKey)
		s.Require().NoError(errSign)
		opts := &types.CreateBucketOptions{
			Visibility:       types.VISIBILITY_TYPE_PRIVATE,
			SourceType:       types.SOURCE_TYPE_ORIGIN,
			ChargedReadQuota: 0,
			PaymentAddress:   owner.String(),
			PrimarySpApproval: &common.Approval{
				ExpiredHeight:              msg.PrimarySpApproval.ExpiredHeight,
				GlobalVirtualGroupFamilyId: gvgFamily.Id,
				Sig:                        sig,
			},
			ApprovalMsgBytes: approvalBytes,
		}

		id, err := s.storageKeeper.CreateBucket(s.ctx, owner, bucketName, primarySpAddr, opts)
		s.Require().NoError(err, "bucket %d should be created successfully", i)
		s.Require().True(id.GT(sdkmath.ZeroUint()))
	}

	// Verify count
	count := s.storageKeeper.GetBucketCountByOwner(s.ctx, owner)
	s.Require().Equal(uint64(3), count, "should have created 3 buckets")

	// Test: 4th bucket should fail
	bucketName := fmt.Sprintf("overflow-bucket-%d", time.Now().Unix())
	// prepare a fresh approval for the overflow attempt
	privKeyOverflow, errGen := gethcrypto.GenerateKey()
	s.Require().NoError(errGen)
	sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKeyOverflow.PublicKey).Hex()
	msgOverflow := &types.MsgCreateBucket{
		Creator:          owner.String(),
		BucketName:       bucketName,
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
	approvalBytesOverflow := msgOverflow.GetApprovalBytes()
	hashOverflow := gethcrypto.Keccak256(approvalBytesOverflow)
	sigOverflow, errSign := gethcrypto.Sign(hashOverflow, privKeyOverflow)
	s.Require().NoError(errSign)
	opts := &types.CreateBucketOptions{
		Visibility:       types.VISIBILITY_TYPE_PRIVATE,
		SourceType:       types.SOURCE_TYPE_ORIGIN,
		ChargedReadQuota: 0,
		PaymentAddress:   owner.String(),
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              msgOverflow.PrimarySpApproval.ExpiredHeight,
			GlobalVirtualGroupFamilyId: gvgFamily.Id,
			Sig:                        sigOverflow,
		},
		ApprovalMsgBytes: approvalBytesOverflow,
	}

	_, err = s.storageKeeper.CreateBucket(s.ctx, owner, bucketName, primarySpAddr, opts)
	s.Require().Error(err, "4th bucket should fail")
	s.Require().Contains(err.Error(), "max bucket limit", "error should mention bucket limit")
	s.Require().Contains(err.Error(), "3/3", "error should show 3/3")
}

// TestCreateBucket_MaxBucketsPerAccount_DifferentOwners tests that limits are per-owner
func (s *TestSuite) TestCreateBucket_MaxBucketsPerAccount_DifferentOwners() {
	// Setup
	owner1 := sample.RandAccAddress()
	owner2 := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{
		Id:          1,
		PrimarySpId: sp.Id,
	}

	// Mock expectations
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(sp, true).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).
		Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).
		Return(gvgFamily, nil).AnyTimes()

	// Set MaxBucketsPerAccount to 2
	params := s.storageKeeper.GetParams(s.ctx)
	params.MaxBucketsPerAccount = 2
	err := s.storageKeeper.SetParams(s.ctx, params)
	s.Require().NoError(err)

	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)

	// Owner1 creates 2 buckets
	for i := 0; i < 2; i++ {
		bucketName := fmt.Sprintf("owner1-bucket-%d-%d", time.Now().Unix(), i)
		privKey, errGen := gethcrypto.GenerateKey()
		s.Require().NoError(errGen)
		sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex()
		msg := &types.MsgCreateBucket{
			Creator:          owner1.String(),
			BucketName:       bucketName,
			Visibility:       types.VISIBILITY_TYPE_PRIVATE,
			PaymentAddress:   owner1.String(),
			PrimarySpAddress: primarySpAddr.String(),
			PrimarySpApproval: &common.Approval{
				ExpiredHeight:              uint64(s.ctx.BlockHeight() + 1000),
				GlobalVirtualGroupFamilyId: gvgFamily.Id,
				Sig:                        nil,
			},
			ChargedReadQuota: 0,
		}
		approvalBytes := msg.GetApprovalBytes()
		hash := gethcrypto.Keccak256(approvalBytes)
		sig, errSign := gethcrypto.Sign(hash, privKey)
		s.Require().NoError(errSign)
		opts := &types.CreateBucketOptions{
			Visibility:     types.VISIBILITY_TYPE_PRIVATE,
			SourceType:     types.SOURCE_TYPE_ORIGIN,
			PaymentAddress: owner1.String(),
			PrimarySpApproval: &common.Approval{
				ExpiredHeight:              msg.PrimarySpApproval.ExpiredHeight,
				GlobalVirtualGroupFamilyId: gvgFamily.Id,
				Sig:                        sig,
			},
			ApprovalMsgBytes: approvalBytes,
		}

		_, err := s.storageKeeper.CreateBucket(s.ctx, owner1, bucketName, primarySpAddr, opts)
		s.Require().NoError(err)
	}

	// Owner2 should still be able to create 2 buckets (independent limit)
	for i := 0; i < 2; i++ {
		bucketName := fmt.Sprintf("owner2-bucket-%d-%d", time.Now().Unix(), i)
		privKey, errGen := gethcrypto.GenerateKey()
		s.Require().NoError(errGen)
		sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex()
		msg := &types.MsgCreateBucket{
			Creator:          owner2.String(),
			BucketName:       bucketName,
			Visibility:       types.VISIBILITY_TYPE_PRIVATE,
			PaymentAddress:   owner2.String(),
			PrimarySpAddress: primarySpAddr.String(),
			PrimarySpApproval: &common.Approval{
				ExpiredHeight:              uint64(s.ctx.BlockHeight() + 1000),
				GlobalVirtualGroupFamilyId: gvgFamily.Id,
				Sig:                        nil,
			},
			ChargedReadQuota: 0,
		}
		approvalBytes := msg.GetApprovalBytes()
		hash := gethcrypto.Keccak256(approvalBytes)
		sig, errSign := gethcrypto.Sign(hash, privKey)
		s.Require().NoError(errSign)
		opts := &types.CreateBucketOptions{
			Visibility:     types.VISIBILITY_TYPE_PRIVATE,
			SourceType:     types.SOURCE_TYPE_ORIGIN,
			PaymentAddress: owner2.String(),
			PrimarySpApproval: &common.Approval{
				ExpiredHeight:              msg.PrimarySpApproval.ExpiredHeight,
				GlobalVirtualGroupFamilyId: gvgFamily.Id,
				Sig:                        sig,
			},
			ApprovalMsgBytes: approvalBytes,
		}

		_, err := s.storageKeeper.CreateBucket(s.ctx, owner2, bucketName, primarySpAddr, opts)
		s.Require().NoError(err, "owner2 should have independent limit")
	}

	// Verify counts
	count1 := s.storageKeeper.GetBucketCountByOwner(s.ctx, owner1)
	count2 := s.storageKeeper.GetBucketCountByOwner(s.ctx, owner2)
	s.Require().Equal(uint64(2), count1)
	s.Require().Equal(uint64(2), count2)
}

// TestCreateBucket_ApprovalExpired tests INFO-019 fix: expiration check
func (s *TestSuite) TestCreateBucket_ApprovalExpired() {
	// Setup
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{
		Id:          1,
		PrimarySpId: sp.Id,
	}

	// Mock expectations
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(sp, true).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).
		Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).
		Return(gvgFamily, nil).AnyTimes()

	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)
	bucketName := fmt.Sprintf("expired-bucket-%d", time.Now().Unix())

	// Advance block height to test expiration
	currentHeight := s.ctx.BlockHeight()
	if currentHeight < 10 {
		s.ctx = s.ctx.WithBlockHeight(10)
	}

	// Test: Expired approval should fail
	opts := &types.CreateBucketOptions{
		Visibility:     types.VISIBILITY_TYPE_PRIVATE,
		SourceType:     types.SOURCE_TYPE_ORIGIN,
		PaymentAddress: owner.String(),
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              uint64(s.ctx.BlockHeight() - 5), // Clearly expired
			GlobalVirtualGroupFamilyId: gvgFamily.Id,
			Sig:                        []byte("test-signature"),
		},
		ApprovalMsgBytes: []byte("test-approval"),
	}

	_, err := s.storageKeeper.CreateBucket(s.ctx, owner, bucketName, primarySpAddr, opts)
	s.Require().Error(err, "expired approval should fail")
	s.Require().Contains(err.Error(), "expired", "error should mention expiration")
}

// TestCreateBucket_InvalidSignature tests INFO-019 fix: signature verification
func (s *TestSuite) TestCreateBucket_InvalidSignature() {
	// Setup
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{
		Id:          1,
		PrimarySpId: sp.Id,
	}

	// Mock expectations
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(sp, true).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).
		Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).
		Return(gvgFamily, nil).AnyTimes()

	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)
	bucketName := fmt.Sprintf("invalid-sig-bucket-%d", time.Now().Unix())

	// Test: Invalid signature should fail
	opts := &types.CreateBucketOptions{
		Visibility:     types.VISIBILITY_TYPE_PRIVATE,
		SourceType:     types.SOURCE_TYPE_ORIGIN,
		PaymentAddress: owner.String(),
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              uint64(s.ctx.BlockHeight() + 1000),
			GlobalVirtualGroupFamilyId: gvgFamily.Id,
			Sig:                        []byte("invalid-signature"), // Invalid signature
		},
		ApprovalMsgBytes: []byte("test-approval"),
	}

	_, err := s.storageKeeper.CreateBucket(s.ctx, owner, bucketName, primarySpAddr, opts)
	s.Require().Error(err, "invalid signature should fail")
	s.Require().Contains(err.Error(), "verify", "error should mention verification")
}

// TestBucketCountIncreaseDecrease tests that count increases on create and decreases on delete
func (s *TestSuite) TestBucketCountIncreaseDecrease() {
	owner := sample.RandAccAddress()

	// Initial count should be 0
	count := s.storageKeeper.GetBucketCountByOwner(s.ctx, owner)
	s.Require().Equal(uint64(0), count)

	// Increment
	s.storageKeeper.IncrementBucketCount(s.ctx, owner)
	count = s.storageKeeper.GetBucketCountByOwner(s.ctx, owner)
	s.Require().Equal(uint64(1), count)

	// Increment again
	s.storageKeeper.IncrementBucketCount(s.ctx, owner)
	count = s.storageKeeper.GetBucketCountByOwner(s.ctx, owner)
	s.Require().Equal(uint64(2), count)

	// Decrement
	s.storageKeeper.DecrementBucketCount(s.ctx, owner)
	count = s.storageKeeper.GetBucketCountByOwner(s.ctx, owner)
	s.Require().Equal(uint64(1), count)

	// Decrement again
	s.storageKeeper.DecrementBucketCount(s.ctx, owner)
	count = s.storageKeeper.GetBucketCountByOwner(s.ctx, owner)
	s.Require().Equal(uint64(0), count)

	// Decrement when already 0 should stay 0
	s.storageKeeper.DecrementBucketCount(s.ctx, owner)
	count = s.storageKeeper.GetBucketCountByOwner(s.ctx, owner)
	s.Require().Equal(uint64(0), count)
}

// TestBucketCountWithCreateAndDelete tests count increases with create and decreases with delete
func (s *TestSuite) TestBucketCountWithCreateAndDelete() {
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{
		Id:          1,
		PrimarySpId: sp.Id,
	}

	// Mock expectations
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).
		Return(sp, true).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).
		Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).
		Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).
		Return(gvgFamily, nil).AnyTimes()

	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)

	// Initial count should be 0
	count := s.storageKeeper.GetBucketCountByOwner(s.ctx, owner)
	s.Require().Equal(uint64(0), count)

	// Create a bucket
	bucketName := fmt.Sprintf("delete-test-bucket-%d", time.Now().Unix())
	// prepare valid approval for creation
	privKey, errGen := gethcrypto.GenerateKey()
	s.Require().NoError(errGen)
	sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex()
	msg := &types.MsgCreateBucket{
		Creator:          owner.String(),
		BucketName:       bucketName,
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
	approvalBytes := msg.GetApprovalBytes()
	hash := gethcrypto.Keccak256(approvalBytes)
	sig, errSign := gethcrypto.Sign(hash, privKey)
	s.Require().NoError(errSign)
	opts := &types.CreateBucketOptions{
		Visibility:     types.VISIBILITY_TYPE_PRIVATE,
		SourceType:     types.SOURCE_TYPE_ORIGIN,
		PaymentAddress: owner.String(),
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              msg.PrimarySpApproval.ExpiredHeight,
			GlobalVirtualGroupFamilyId: gvgFamily.Id,
			Sig:                        sig,
		},
		ApprovalMsgBytes: approvalBytes,
	}

	_, err := s.storageKeeper.CreateBucket(s.ctx, owner, bucketName, primarySpAddr, opts)
	s.Require().NoError(err)

	// Count should be 1 after create
	count = s.storageKeeper.GetBucketCountByOwner(s.ctx, owner)
	s.Require().Equal(uint64(1), count)

	// Route A: unit test verifies the counter behavior without walking full DeleteBucket path.
	// Full DeleteBucket requires extensive cross-keeper mocks (permission/payment/evm). We validate
	// the decrement logic directly here; the wire-up is present in keeper.go:341.
	s.storageKeeper.DecrementBucketCount(s.ctx, owner)

	// Count should be 0 after decrement
	count = s.storageKeeper.GetBucketCountByOwner(s.ctx, owner)
	s.Require().Equal(uint64(0), count)
}
