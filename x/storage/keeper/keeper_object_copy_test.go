package keeper_test

import (
	"crypto/ecdsa"
	"fmt"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	gethcrypto "github.com/ethereum/go-ethereum/crypto"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/types/common"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	types "github.com/mocachain/moca/v2/x/storage/types"
	vgtypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// LOW-017: CopyObject MUST validate PrimarySpApproval (nil/expired/invalid signature should fail)

func (s *TestSuite) TestLOW017_CopyObject_NilApproval_MustFail() {
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{Id: 1, PrimarySpId: sp.Id}

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), gomock.Any()).Return(sp).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).Return(gvgFamily, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SettleAndDistributeGVGFamily(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	// required by SealEmptyObjectOnVirtualGroup path
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroupFamily{
		Id:                    gvgFamily.Id,
		PrimarySpId:           sp.Id,
		GlobalVirtualGroupIds: []uint32{1},
	}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroup{
		Id:             1,
		FamilyId:       gvgFamily.Id,
		PrimarySpId:    sp.Id,
		SecondarySpIds: []uint32{},
	}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroup{
		Id:             1,
		FamilyId:       gvgFamily.Id,
		PrimarySpId:    sp.Id,
		SecondarySpIds: []uint32{},
	}, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(0),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(0),
		SecondaryStorePrice: sdkmath.LegacyNewDec(0),
	}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
	// permission check will pass because operator is the bucket/object owner

	// Generate a keypair for the SP approval address so we can sign
	privKey, errGen := gethcrypto.GenerateKey()
	s.Require().NoError(errGen)
	sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex()

	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)
	s.ctx = s.ctx.WithBlockHeight(100)
	// ensure versioned storage params exist at timestamp strictly less than the upcoming ts
	oldCtx := s.ctx.WithBlockTime(s.ctx.BlockTime().Add(-1 * time.Second))
	s.Require().NoError(s.storageKeeper.SetVersionedParamsWithTS(oldCtx, types.DefaultParams().VersionedParams))

	// create src and dst buckets
	srcBucket := fmt.Sprintf("src-%d", time.Now().UnixNano())
	dstBucket := fmt.Sprintf("dst-%d", time.Now().UnixNano())

	for _, bucket := range []string{srcBucket, dstBucket} {
		msg := &types.MsgCreateBucket{
			Creator:          owner.String(),
			BucketName:       bucket,
			Visibility:       types.VISIBILITY_TYPE_PRIVATE,
			PaymentAddress:   owner.String(),
			PrimarySpAddress: primarySpAddr.String(),
			PrimarySpApproval: &common.Approval{
				ExpiredHeight:              uint64(s.ctx.BlockHeight() + 1000),
				GlobalVirtualGroupFamilyId: gvgFamily.Id,
			},
		}
		approvalBytes := msg.GetApprovalBytes()
		sig, errSign := gethcrypto.Sign(gethcrypto.Keccak256(approvalBytes), privKey)
		s.Require().NoError(errSign)
		msg.PrimarySpApproval.Sig = sig

		_, err := s.storageKeeper.CreateBucket(s.ctx, owner, bucket, primarySpAddr, &types.CreateBucketOptions{
			Visibility:        types.VISIBILITY_TYPE_PRIVATE,
			SourceType:        types.SOURCE_TYPE_ORIGIN,
			ChargedReadQuota:  0,
			PaymentAddress:    owner.String(),
			PrimarySpApproval: msg.PrimarySpApproval,
			ApprovalMsgBytes:  approvalBytes,
		})
		s.Require().NoError(err)
	}

	// create empty object in src
	srcObject := "obj.txt"
	_, err := s.storageKeeper.CreateObject(s.ctx, owner, srcBucket, srcObject, 0, types.CreateObjectOptions{
		SourceType: types.SOURCE_TYPE_ORIGIN,
		Visibility: types.VISIBILITY_TYPE_PRIVATE,
	})
	s.Require().NoError(err)

	// nil approval must fail
	_, err = s.storageKeeper.CopyObject(s.ctx, owner, srcBucket, srcObject, dstBucket, "copy-nil.txt", types.CopyObjectOptions{
		SourceType:        types.SOURCE_TYPE_ORIGIN,
		Visibility:        types.VISIBILITY_TYPE_PRIVATE,
		PrimarySpApproval: nil,
		ApprovalMsgBytes:  []byte("approval-bytes"),
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestLOW017_CopyObject_ExpiredApproval_MustFail() {
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{Id: 1, PrimarySpId: sp.Id}

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), gomock.Any()).Return(sp).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).Return(gvgFamily, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SettleAndDistributeGVGFamily(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroupFamily{
		Id:                    gvgFamily.Id,
		PrimarySpId:           sp.Id,
		GlobalVirtualGroupIds: []uint32{1},
	}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroup{
		Id:             1,
		FamilyId:       gvgFamily.Id,
		PrimarySpId:    sp.Id,
		SecondarySpIds: []uint32{},
	}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroup{
		Id:             1,
		FamilyId:       gvgFamily.Id,
		PrimarySpId:    sp.Id,
		SecondarySpIds: []uint32{},
	}, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(0),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(0),
		SecondaryStorePrice: sdkmath.LegacyNewDec(0),
	}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
	// permission check will pass because operator is the bucket/object owner

	privKey2, errGen2 := gethcrypto.GenerateKey()
	s.Require().NoError(errGen2)
	sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey2.PublicKey).Hex()

	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)
	s.ctx = s.ctx.WithBlockHeight(200)
	oldCtx := s.ctx.WithBlockTime(s.ctx.BlockTime().Add(-1 * time.Second))
	s.Require().NoError(s.storageKeeper.SetVersionedParamsWithTS(oldCtx, types.DefaultParams().VersionedParams))

	srcBucket := fmt.Sprintf("src-%d", time.Now().UnixNano())
	dstBucket := fmt.Sprintf("dst-%d", time.Now().UnixNano())

	for _, bucket := range []string{srcBucket, dstBucket} {
		msg := &types.MsgCreateBucket{
			Creator:          owner.String(),
			BucketName:       bucket,
			Visibility:       types.VISIBILITY_TYPE_PRIVATE,
			PaymentAddress:   owner.String(),
			PrimarySpAddress: primarySpAddr.String(),
			PrimarySpApproval: &common.Approval{
				ExpiredHeight:              uint64(s.ctx.BlockHeight() + 1000),
				GlobalVirtualGroupFamilyId: gvgFamily.Id,
			},
		}
		approvalBytes := msg.GetApprovalBytes()
		sig, errSign := gethcrypto.Sign(gethcrypto.Keccak256(approvalBytes), privKey2)
		s.Require().NoError(errSign)
		msg.PrimarySpApproval.Sig = sig

		_, err := s.storageKeeper.CreateBucket(s.ctx, owner, bucket, primarySpAddr, &types.CreateBucketOptions{
			Visibility:        types.VISIBILITY_TYPE_PRIVATE,
			SourceType:        types.SOURCE_TYPE_ORIGIN,
			ChargedReadQuota:  0,
			PaymentAddress:    owner.String(),
			PrimarySpApproval: msg.PrimarySpApproval,
			ApprovalMsgBytes:  approvalBytes,
		})
		s.Require().NoError(err)
	}

	srcObject := "obj2.txt"
	_, err := s.storageKeeper.CreateObject(s.ctx, owner, srcBucket, srcObject, 0, types.CreateObjectOptions{
		SourceType: types.SOURCE_TYPE_ORIGIN,
		Visibility: types.VISIBILITY_TYPE_PRIVATE,
	})
	s.Require().NoError(err)

	// expired approval must fail
	_, err = s.storageKeeper.CopyObject(s.ctx, owner, srcBucket, srcObject, dstBucket, "copy-expired.txt", types.CopyObjectOptions{
		SourceType: types.SOURCE_TYPE_ORIGIN,
		Visibility: types.VISIBILITY_TYPE_PRIVATE,
		PrimarySpApproval: &common.Approval{
			ExpiredHeight: uint64(s.ctx.BlockHeight() - 1),
			Sig:           []byte("fake"),
		},
		ApprovalMsgBytes: []byte("approval-bytes"),
	})
	s.Require().Error(err)
}

func (s *TestSuite) TestLOW017_CopyObject_InvalidSignature_MustFail() {
	owner := sample.RandAccAddress()
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{Id: 1, PrimarySpId: sp.Id}

	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), gomock.Any()).Return(sp).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).Return(gvgFamily, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SettleAndDistributeGVGFamily(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroupFamily{
		Id:                    gvgFamily.Id,
		PrimarySpId:           sp.Id,
		GlobalVirtualGroupIds: []uint32{1},
	}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroup{
		Id:             1,
		FamilyId:       gvgFamily.Id,
		PrimarySpId:    sp.Id,
		SecondarySpIds: []uint32{},
	}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroup{
		Id:             1,
		FamilyId:       gvgFamily.Id,
		PrimarySpId:    sp.Id,
		SecondarySpIds: []uint32{},
	}, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(0),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(0),
		SecondaryStorePrice: sdkmath.LegacyNewDec(0),
	}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
	// permission check will pass because operator is the bucket/object owner

	privKey3, errGen3 := gethcrypto.GenerateKey()
	s.Require().NoError(errGen3)
	sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey3.PublicKey).Hex()

	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)
	s.ctx = s.ctx.WithBlockHeight(300)
	oldCtx := s.ctx.WithBlockTime(s.ctx.BlockTime().Add(-1 * time.Second))
	s.Require().NoError(s.storageKeeper.SetVersionedParamsWithTS(oldCtx, types.DefaultParams().VersionedParams))

	srcBucket := fmt.Sprintf("src-%d", time.Now().UnixNano())
	dstBucket := fmt.Sprintf("dst-%d", time.Now().UnixNano())

	for _, bucket := range []string{srcBucket, dstBucket} {
		msg := &types.MsgCreateBucket{
			Creator:          owner.String(),
			BucketName:       bucket,
			Visibility:       types.VISIBILITY_TYPE_PRIVATE,
			PaymentAddress:   owner.String(),
			PrimarySpAddress: primarySpAddr.String(),
			PrimarySpApproval: &common.Approval{
				ExpiredHeight:              uint64(s.ctx.BlockHeight() + 1000),
				GlobalVirtualGroupFamilyId: gvgFamily.Id,
			},
		}
		approvalBytes := msg.GetApprovalBytes()
		sig, errSign := gethcrypto.Sign(gethcrypto.Keccak256(approvalBytes), privKey3)
		s.Require().NoError(errSign)
		msg.PrimarySpApproval.Sig = sig

		_, err := s.storageKeeper.CreateBucket(s.ctx, owner, bucket, primarySpAddr, &types.CreateBucketOptions{
			Visibility:        types.VISIBILITY_TYPE_PRIVATE,
			SourceType:        types.SOURCE_TYPE_ORIGIN,
			ChargedReadQuota:  0,
			PaymentAddress:    owner.String(),
			PrimarySpApproval: msg.PrimarySpApproval,
			ApprovalMsgBytes:  approvalBytes,
		})
		s.Require().NoError(err)
	}

	srcObject := "obj3.txt"
	_, err := s.storageKeeper.CreateObject(s.ctx, owner, srcBucket, srcObject, 0, types.CreateObjectOptions{
		SourceType: types.SOURCE_TYPE_ORIGIN,
		Visibility: types.VISIBILITY_TYPE_PRIVATE,
	})
	s.Require().NoError(err)

	// invalid signature must fail
	_, err = s.storageKeeper.CopyObject(s.ctx, owner, srcBucket, srcObject, dstBucket, "copy-badsig.txt", types.CopyObjectOptions{
		SourceType: types.SOURCE_TYPE_ORIGIN,
		Visibility: types.VISIBILITY_TYPE_PRIVATE,
		PrimarySpApproval: &common.Approval{
			ExpiredHeight: uint64(s.ctx.BlockHeight() + 1000),
			Sig:           []byte{0x00},
		},
		ApprovalMsgBytes: []byte("approval-bytes"),
	})
	s.Require().Error(err)
}

// A tiny compile-time guard for the suite to ensure tests run via `go test ./x/storage/keeper -run LOW017` if needed.
func TestLOW017_Placeholder(t *testing.T) {}

// MOCA-1421: CopyObject writes into the destination bucket, so it must apply the same
// destination-side guards CreateObject does — a create permission on the destination bucket,
// a name-collision reject, and the destination bucket owner as the object owner.

// setupCopyObjectDeps wires the mock keepers a CopyObject scenario touches: bucket/object
// creation, the empty-object seal path, and store-fee billing.
func (s *TestSuite) setupCopyObjectDeps(sp *sptypes.StorageProvider, gvgFamily *vgtypes.GlobalVirtualGroupFamily) {
	s.spKeeper.EXPECT().GetStorageProviderByOperatorAddr(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Any()).Return(sp, true).AnyTimes()
	s.spKeeper.EXPECT().MustGetStorageProvider(gomock.Any(), gomock.Any()).Return(sp).AnyTimes()
	s.accountKeeper.EXPECT().GetAccount(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.accountKeeper.EXPECT().GetSequence(gomock.Any(), gomock.Any()).Return(uint64(0), nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetAndCheckGVGFamilyAvailableForNewBucket(gomock.Any(), gomock.Any()).Return(gvgFamily, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SettleAndDistributeGVGFamily(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroupFamily{
		Id:                    gvgFamily.Id,
		PrimarySpId:           sp.Id,
		GlobalVirtualGroupIds: []uint32{1},
	}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroup{
		Id:             1,
		FamilyId:       gvgFamily.Id,
		PrimarySpId:    sp.Id,
		SecondarySpIds: []uint32{},
	}, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGlobalVirtualGroupIfAvailable(gomock.Any(), gomock.Any(), gomock.Any()).Return(&vgtypes.GlobalVirtualGroup{
		Id:             1,
		FamilyId:       gvgFamily.Id,
		PrimarySpId:    sp.Id,
		SecondarySpIds: []uint32{},
	}, nil).AnyTimes()
	s.virtualGroupKeeper.EXPECT().SetGVGAndEmitUpdateEvent(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(0),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(0),
		SecondaryStorePrice: sdkmath.LegacyNewDec(0),
	}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	s.paymentKeeper.EXPECT().MergeOutFlows(gomock.Any()).Return([]paymenttypes.OutFlow{}).AnyTimes()
}

// createBucketForCopy creates a private bucket owned by owner, signing the primary-SP approval
// with the SP's approval key so the create passes.
func (s *TestSuite) createBucketForCopy(owner sdk.AccAddress, bucket string, sp *sptypes.StorageProvider, privKey *ecdsa.PrivateKey, gvgFamily *vgtypes.GlobalVirtualGroupFamily) {
	primarySpAddr := sdk.MustAccAddressFromHex(sp.OperatorAddress)
	msg := &types.MsgCreateBucket{
		Creator:          owner.String(),
		BucketName:       bucket,
		Visibility:       types.VISIBILITY_TYPE_PRIVATE,
		PaymentAddress:   owner.String(),
		PrimarySpAddress: primarySpAddr.String(),
		PrimarySpApproval: &common.Approval{
			ExpiredHeight:              uint64(s.ctx.BlockHeight() + 1000),
			GlobalVirtualGroupFamilyId: gvgFamily.Id,
		},
	}
	approvalBytes := msg.GetApprovalBytes()
	sig, err := gethcrypto.Sign(gethcrypto.Keccak256(approvalBytes), privKey)
	s.Require().NoError(err)
	msg.PrimarySpApproval.Sig = sig

	_, err = s.storageKeeper.CreateBucket(s.ctx, owner, bucket, primarySpAddr, &types.CreateBucketOptions{
		Visibility:        types.VISIBILITY_TYPE_PRIVATE,
		SourceType:        types.SOURCE_TYPE_ORIGIN,
		ChargedReadQuota:  0,
		PaymentAddress:    owner.String(),
		PrimarySpApproval: msg.PrimarySpApproval,
		ApprovalMsgBytes:  approvalBytes,
	})
	s.Require().NoError(err)
}

// signCopyApproval returns the approval bytes plus a matching primary-SP approval signed by privKey.
func (s *TestSuite) signCopyApproval(privKey *ecdsa.PrivateKey) ([]byte, *common.Approval) {
	approvalBytes := []byte("copy-object-approval")
	sig, err := gethcrypto.Sign(gethcrypto.Keccak256(approvalBytes), privKey)
	s.Require().NoError(err)
	return approvalBytes, &common.Approval{
		ExpiredHeight: uint64(s.ctx.BlockHeight() + 1000),
		Sig:           sig,
	}
}

func (s *TestSuite) newCopyObjectSP() (*sptypes.StorageProvider, *vgtypes.GlobalVirtualGroupFamily, *ecdsa.PrivateKey) {
	sp := &sptypes.StorageProvider{
		Id:              1,
		OperatorAddress: sample.RandAccAddress().String(),
		Status:          sptypes.STATUS_IN_SERVICE,
		ApprovalAddress: sample.RandAccAddress().String(),
	}
	gvgFamily := &vgtypes.GlobalVirtualGroupFamily{Id: 1, PrimarySpId: sp.Id}
	s.setupCopyObjectDeps(sp, gvgFamily)

	privKey, err := gethcrypto.GenerateKey()
	s.Require().NoError(err)
	// point the SP approval key at the generated keypair so signed approvals verify
	sp.ApprovalAddress = gethcrypto.PubkeyToAddress(privKey.PublicKey).Hex()

	s.ctx = s.ctx.WithBlockHeight(100)
	oldCtx := s.ctx.WithBlockTime(s.ctx.BlockTime().Add(-1 * time.Second))
	s.Require().NoError(s.storageKeeper.SetVersionedParamsWithTS(oldCtx, types.DefaultParams().VersionedParams))
	return sp, gvgFamily, privKey
}

// TestCopyObject_NoDstBucketPermission_MustFail: the operator owns the source (so the source
// copy check passes) but holds no permission on a destination bucket owned by someone else.
// Before the destination-side check this copied into the foreign bucket; it must now be denied.
func (s *TestSuite) TestCopyObject_NoDstBucketPermission_MustFail() {
	srcOwner := sample.RandAccAddress()
	dstOwner := sample.RandAccAddress()
	sp, gvgFamily, privKey := s.newCopyObjectSP()

	// the destination bucket grants the operator no account or group policy
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	srcBucket := fmt.Sprintf("src-%d", time.Now().UnixNano())
	dstBucket := fmt.Sprintf("dst-%d", time.Now().UnixNano())
	s.createBucketForCopy(srcOwner, srcBucket, sp, privKey, gvgFamily)
	s.createBucketForCopy(dstOwner, dstBucket, sp, privKey, gvgFamily)

	srcObject := "obj.txt"
	_, err := s.storageKeeper.CreateObject(s.ctx, srcOwner, srcBucket, srcObject, 0, types.CreateObjectOptions{
		SourceType: types.SOURCE_TYPE_ORIGIN,
		Visibility: types.VISIBILITY_TYPE_PRIVATE,
	})
	s.Require().NoError(err)

	approvalBytes, approval := s.signCopyApproval(privKey)
	_, err = s.storageKeeper.CopyObject(s.ctx, srcOwner, srcBucket, srcObject, dstBucket, "copy.txt", types.CopyObjectOptions{
		SourceType:        types.SOURCE_TYPE_ORIGIN,
		Visibility:        types.VISIBILITY_TYPE_PRIVATE,
		PrimarySpApproval: approval,
		ApprovalMsgBytes:  approvalBytes,
	})
	s.Require().ErrorIs(err, types.ErrAccessDenied)
}

// TestCopyObject_DstNameCollision_MustFail: copying onto a name that already exists must be
// rejected instead of silently overwriting (which orphaned the previous object's record).
func (s *TestSuite) TestCopyObject_DstNameCollision_MustFail() {
	owner := sample.RandAccAddress()
	sp, gvgFamily, privKey := s.newCopyObjectSP()

	// operator owns both buckets, so permission passes via ownership
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()
	s.permissionKeeper.EXPECT().GetPolicyGroupForResource(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, false).AnyTimes()

	srcBucket := fmt.Sprintf("src-%d", time.Now().UnixNano())
	dstBucket := fmt.Sprintf("dst-%d", time.Now().UnixNano())
	s.createBucketForCopy(owner, srcBucket, sp, privKey, gvgFamily)
	s.createBucketForCopy(owner, dstBucket, sp, privKey, gvgFamily)

	_, err := s.storageKeeper.CreateObject(s.ctx, owner, srcBucket, "obj.txt", 0, types.CreateObjectOptions{
		SourceType: types.SOURCE_TYPE_ORIGIN,
		Visibility: types.VISIBILITY_TYPE_PRIVATE,
	})
	s.Require().NoError(err)

	// a different object already occupies the destination name
	_, err = s.storageKeeper.CreateObject(s.ctx, owner, dstBucket, "taken.txt", 0, types.CreateObjectOptions{
		SourceType: types.SOURCE_TYPE_ORIGIN,
		Visibility: types.VISIBILITY_TYPE_PRIVATE,
	})
	s.Require().NoError(err)

	approvalBytes, approval := s.signCopyApproval(privKey)
	_, err = s.storageKeeper.CopyObject(s.ctx, owner, srcBucket, "obj.txt", dstBucket, "taken.txt", types.CopyObjectOptions{
		SourceType:        types.SOURCE_TYPE_ORIGIN,
		Visibility:        types.VISIBILITY_TYPE_PRIVATE,
		PrimarySpApproval: approval,
		ApprovalMsgBytes:  approvalBytes,
	})
	s.Require().ErrorIs(err, types.ErrObjectAlreadyExists)
}

// TestCopyObject_OwnerIsDstBucketOwner: the copied object is owned by the destination bucket
// owner, not the operator that requested the copy. The operator here is authorized on the
// destination via an account policy but is not its owner, so the two are distinguishable.
func (s *TestSuite) TestCopyObject_OwnerIsDstBucketOwner() {
	srcOwner := sample.RandAccAddress()
	dstOwner := sample.RandAccAddress()
	sp, gvgFamily, privKey := s.newCopyObjectSP()

	// grant the operator CreateObject on the destination bucket
	allowPolicy := &permtypes.Policy{
		Statements: []*permtypes.Statement{{
			Effect:  permtypes.EFFECT_ALLOW,
			Actions: []permtypes.ActionType{permtypes.ACTION_CREATE_OBJECT},
		}},
	}
	s.permissionKeeper.EXPECT().GetPolicyForAccount(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).Return(allowPolicy, true).AnyTimes()

	srcBucket := fmt.Sprintf("src-%d", time.Now().UnixNano())
	dstBucket := fmt.Sprintf("dst-%d", time.Now().UnixNano())
	s.createBucketForCopy(srcOwner, srcBucket, sp, privKey, gvgFamily)
	s.createBucketForCopy(dstOwner, dstBucket, sp, privKey, gvgFamily)

	_, err := s.storageKeeper.CreateObject(s.ctx, srcOwner, srcBucket, "obj.txt", 0, types.CreateObjectOptions{
		SourceType: types.SOURCE_TYPE_ORIGIN,
		Visibility: types.VISIBILITY_TYPE_PRIVATE,
	})
	s.Require().NoError(err)

	approvalBytes, approval := s.signCopyApproval(privKey)
	_, err = s.storageKeeper.CopyObject(s.ctx, srcOwner, srcBucket, "obj.txt", dstBucket, "copy.txt", types.CopyObjectOptions{
		SourceType:        types.SOURCE_TYPE_ORIGIN,
		Visibility:        types.VISIBILITY_TYPE_PRIVATE,
		PrimarySpApproval: approval,
		ApprovalMsgBytes:  approvalBytes,
	})
	s.Require().NoError(err)

	copied, found := s.storageKeeper.GetObjectInfo(s.ctx, dstBucket, "copy.txt")
	s.Require().True(found)
	s.Require().Equal(dstOwner.String(), copied.Owner)
	s.Require().NotEqual(srcOwner.String(), copied.Owner)
}
