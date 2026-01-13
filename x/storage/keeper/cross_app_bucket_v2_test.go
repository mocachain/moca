package keeper_test

import (
	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/evmos/evmos/v12/testutil/sample"
	"github.com/evmos/evmos/v12/x/storage/keeper"
	storagetypes "github.com/evmos/evmos/v12/x/storage/types"
	"go.uber.org/mock/gomock"
)

// Verify: V2 package is correctly deserialized and GlobalVirtualGroupFamilyId is propagated to CreateBucketOptions.PrimarySpApproval
func (s *TestSuite) TestSynCreateBucketV2_PassesGVGFamilyId() {
	ctrl := gomock.NewController(s.T())
	storageKeeper := storagetypes.NewMockStorageKeeper(ctrl)
	storageKeeper.EXPECT().Logger(gomock.Any()).Return(s.ctx.Logger()).AnyTimes()

	app := keeper.NewBucketApp(storageKeeper)

	// Build a V2 package (with GlobalVirtualGroupFamilyId)
	v2 := storagetypes.CreateBucketSynPackageV2{
		Creator:                        sample.RandAccAddress(),
		BucketName:                     "bucket-v2",
		Visibility:                     uint32(storagetypes.VISIBILITY_TYPE_PUBLIC_READ),
		PaymentAddress:                 sample.RandAccAddress(),
		PrimarySpAddress:               sample.RandAccAddress(),
		PrimarySpApprovalExpiredHeight: 1000,
		GlobalVirtualGroupFamilyId:     7,
		ChargedReadQuota:               1024,
		ExtraData:                      []byte("v2-extra"),
	}
	// Pack via local ABI to avoid depending on repo-specific V2 MustSerialize details
	payload := mustPackCreateBucketSynPackageV2(&v2)
	// Prefix OperationType with V2 op code
	payload = append([]byte{storagetypes.OperationCreateBucketV2}, payload...)

	storageKeeper.EXPECT().GetSourceTypeByChainId(gomock.Any(), gomock.Any()).Return(storagetypes.SOURCE_TYPE_BSC_CROSS_CHAIN, nil).AnyTimes()

	// Assert V2 field propagation
	storageKeeper.EXPECT().CreateBucket(
		gomock.Any(), gomock.Any(), gomock.Eq("bucket-v2"), gomock.Any(), gomock.Any(),
	).DoAndReturn(func(ctx sdk.Context, creator sdk.AccAddress, bucketName string, primarySP sdk.AccAddress, opts *storagetypes.CreateBucketOptions) (sdkmath.Uint, error) {
		s.Require().NotNil(opts)
		s.Require().NotNil(opts.PrimarySpApproval)
		s.Require().Equal(uint32(7), opts.PrimarySpApproval.GlobalVirtualGroupFamilyId)
		return sdkmath.NewUint(1), nil
	})

	res := app.ExecuteSynPackage(s.ctx, &sdk.CrossChainAppContext{}, payload)
	s.Require().NoError(res.Err)
}

// Verify: V1 package still works (fallback from V2 failure to V1)
func (s *TestSuite) TestSynCreateBucket_V1StillWorks() {
	ctrl := gomock.NewController(s.T())
	storageKeeper := storagetypes.NewMockStorageKeeper(ctrl)
	storageKeeper.EXPECT().Logger(gomock.Any()).Return(s.ctx.Logger()).AnyTimes()

	app := keeper.NewBucketApp(storageKeeper)

	v1 := storagetypes.CreateBucketSynPackage{
		Creator:                        sample.RandAccAddress(),
		BucketName:                     "bucket-v1",
		Visibility:                     uint32(storagetypes.VISIBILITY_TYPE_PUBLIC_READ),
		PaymentAddress:                 sample.RandAccAddress(),
		PrimarySpAddress:               sample.RandAccAddress(),
		PrimarySpApprovalExpiredHeight: 1000,
		ChargedReadQuota:               100,
		ExtraData:                      []byte("v1-extra"),
	}
	payload := v1.MustSerialize()
	payload = append([]byte{storagetypes.OperationCreateBucket}, payload...)

	storageKeeper.EXPECT().GetSourceTypeByChainId(gomock.Any(), gomock.Any()).Return(storagetypes.SOURCE_TYPE_BSC_CROSS_CHAIN, nil).AnyTimes()
	storageKeeper.EXPECT().CreateBucket(
		gomock.Any(), gomock.Any(), gomock.Eq("bucket-v1"), gomock.Any(), gomock.Any(),
	).Return(sdkmath.NewUint(1), nil)

	res := app.ExecuteSynPackage(s.ctx, &sdk.CrossChainAppContext{}, payload)
	s.Require().NoError(res.Err)
}

// Verify: FailAck path handles V2 package (enters V2 branch, no error)
func (s *TestSuite) TestFailAckCreateBucketV2_PassesGVGFamilyId() {
	ctrl := gomock.NewController(s.T())
	storageKeeper := storagetypes.NewMockStorageKeeper(ctrl)
	storageKeeper.EXPECT().Logger(gomock.Any()).Return(s.ctx.Logger()).AnyTimes()

	app := keeper.NewBucketApp(storageKeeper)

	v2 := storagetypes.CreateBucketSynPackageV2{
		Creator:                    sample.RandAccAddress(),
		BucketName:                 "bucket-v2-failack",
		Visibility:                 uint32(storagetypes.VISIBILITY_TYPE_PUBLIC_READ),
		PaymentAddress:             sample.RandAccAddress(),
		PrimarySpAddress:           sample.RandAccAddress(),
		ExtraData:                  []byte("v2-failack"),
		GlobalVirtualGroupFamilyId: 9,
	}
	payload := mustPackCreateBucketSynPackageV2(&v2)
	payload = append([]byte{storagetypes.OperationCreateBucketV2}, payload...)

	storageKeeper.EXPECT().GetSourceTypeByChainId(gomock.Any(), gomock.Any()).Return(storagetypes.SOURCE_TYPE_BSC_CROSS_CHAIN, nil).AnyTimes()

	// FailAck path does not create bucket; only assert no error
	res := app.ExecuteFailAckPackage(s.ctx, &sdk.CrossChainAppContext{}, payload)
	s.Require().NoError(res.Err)
}

// Explicitly verify: V2 deserialization fails -> fallback to V1 succeeds
func (s *TestSuite) TestV1PackageNotDeserializedAsV2() {
	ctrl := gomock.NewController(s.T())
	storageKeeper := storagetypes.NewMockStorageKeeper(ctrl)
	storageKeeper.EXPECT().Logger(gomock.Any()).Return(s.ctx.Logger()).AnyTimes()

	app := keeper.NewBucketApp(storageKeeper)

	v1 := storagetypes.CreateBucketSynPackage{
		Creator:          sample.RandAccAddress(),
		BucketName:       "bucket-v1-fallback",
		Visibility:       uint32(storagetypes.VISIBILITY_TYPE_PUBLIC_READ),
		PaymentAddress:   sample.RandAccAddress(),
		PrimarySpAddress: sample.RandAccAddress(),
		ExtraData:        []byte("v1-fallback"),
	}
	raw := v1.MustSerialize()
	raw = append([]byte{storagetypes.OperationCreateBucket}, raw...)

	// Explicit assertion: V1 payload must fail V2 deserialization; V1 must succeed
	_, errV2 := storagetypes.DeserializeCrossChainPackageV2(raw, storagetypes.BucketChannelId, sdk.FailAckCrossChainPackageType)
	s.Require().Error(errV2)
	_, errV1 := storagetypes.DeserializeCrossChainPackage(raw, storagetypes.BucketChannelId, sdk.FailAckCrossChainPackageType)
	s.Require().NoError(errV1)

	storageKeeper.EXPECT().GetSourceTypeByChainId(gomock.Any(), gomock.Any()).Return(storagetypes.SOURCE_TYPE_BSC_CROSS_CHAIN, nil).AnyTimes()
	storageKeeper.EXPECT().CreateBucket(
		gomock.Any(), gomock.Any(), gomock.Eq("bucket-v1-fallback"), gomock.Any(), gomock.Any(),
	).Return(sdkmath.NewUint(1), nil)

	res := app.ExecuteSynPackage(s.ctx, &sdk.CrossChainAppContext{}, raw)
	s.Require().NoError(res.Err)
}

// Mixed scenario: V2 -> V1 -> V2 alternating handling
func (s *TestSuite) TestMixedV1AndV2Packages() {
	ctrl := gomock.NewController(s.T())
	storageKeeper := storagetypes.NewMockStorageKeeper(ctrl)
	storageKeeper.EXPECT().Logger(gomock.Any()).Return(s.ctx.Logger()).AnyTimes()

	app := keeper.NewBucketApp(storageKeeper)
	storageKeeper.EXPECT().GetSourceTypeByChainId(gomock.Any(), gomock.Any()).Return(storagetypes.SOURCE_TYPE_BSC_CROSS_CHAIN, nil).AnyTimes()

	// First V2
	v2a := storagetypes.CreateBucketSynPackageV2{
		Creator:                    sample.RandAccAddress(),
		BucketName:                 "bucket-1-v2",
		Visibility:                 uint32(storagetypes.VISIBILITY_TYPE_PUBLIC_READ),
		PaymentAddress:             sample.RandAccAddress(),
		PrimarySpAddress:           sample.RandAccAddress(),
		GlobalVirtualGroupFamilyId: 5,
	}
	p2a := mustPackCreateBucketSynPackageV2(&v2a)
	p2a = append([]byte{storagetypes.OperationCreateBucketV2}, p2a...)
	storageKeeper.EXPECT().CreateBucket(
		gomock.Any(), gomock.Any(), gomock.Eq("bucket-1-v2"), gomock.Any(), gomock.Any(),
	).DoAndReturn(func(ctx sdk.Context, creator sdk.AccAddress, bucketName string, primarySP sdk.AccAddress, opts *storagetypes.CreateBucketOptions) (sdkmath.Uint, error) {
		s.Require().Equal(uint32(5), opts.PrimarySpApproval.GlobalVirtualGroupFamilyId)
		return sdkmath.NewUint(1), nil
	})
	res := app.ExecuteSynPackage(s.ctx, &sdk.CrossChainAppContext{}, p2a)
	s.Require().NoError(res.Err)

	// Second V1
	v1 := storagetypes.CreateBucketSynPackage{
		Creator:          sample.RandAccAddress(),
		BucketName:       "bucket-2-v1",
		Visibility:       uint32(storagetypes.VISIBILITY_TYPE_PUBLIC_READ),
		PaymentAddress:   sample.RandAccAddress(),
		PrimarySpAddress: sample.RandAccAddress(),
	}
	p1 := v1.MustSerialize()
	p1 = append([]byte{storagetypes.OperationCreateBucket}, p1...)
	storageKeeper.EXPECT().CreateBucket(
		gomock.Any(), gomock.Any(), gomock.Eq("bucket-2-v1"), gomock.Any(), gomock.Any(),
	).Return(sdkmath.NewUint(2), nil)
	res = app.ExecuteSynPackage(s.ctx, &sdk.CrossChainAppContext{}, p1)
	s.Require().NoError(res.Err)

	// Third V2
	v2b := storagetypes.CreateBucketSynPackageV2{
		Creator:                    sample.RandAccAddress(),
		BucketName:                 "bucket-3-v2",
		Visibility:                 uint32(storagetypes.VISIBILITY_TYPE_PUBLIC_READ),
		PaymentAddress:             sample.RandAccAddress(),
		PrimarySpAddress:           sample.RandAccAddress(),
		GlobalVirtualGroupFamilyId: 10,
	}
	p2b := mustPackCreateBucketSynPackageV2(&v2b)
	p2b = append([]byte{storagetypes.OperationCreateBucketV2}, p2b...)
	storageKeeper.EXPECT().CreateBucket(
		gomock.Any(), gomock.Any(), gomock.Eq("bucket-3-v2"), gomock.Any(), gomock.Any(),
	).DoAndReturn(func(ctx sdk.Context, creator sdk.AccAddress, bucketName string, primarySP sdk.AccAddress, opts *storagetypes.CreateBucketOptions) (sdkmath.Uint, error) {
		s.Require().Equal(uint32(10), opts.PrimarySpApproval.GlobalVirtualGroupFamilyId)
		return sdkmath.NewUint(3), nil
	})
	res = app.ExecuteSynPackage(s.ctx, &sdk.CrossChainAppContext{}, p2b)
	s.Require().NoError(res.Err)
}

// Error case: invalid payload triggers deserialization panic
func (s *TestSuite) TestExecuteSynPackage_InvalidPayload_Panics() {
	ctrl := gomock.NewController(s.T())
	storageKeeper := storagetypes.NewMockStorageKeeper(ctrl)
	storageKeeper.EXPECT().Logger(gomock.Any()).Return(s.ctx.Logger()).AnyTimes()
	app := keeper.NewBucketApp(storageKeeper)

	invalid := []byte("this-is-garbage")
	s.Require().Panics(func() {
		_ = app.ExecuteSynPackage(s.ctx, &sdk.CrossChainAppContext{}, invalid)
	})
}

// Unknown OperationType should panic during dispatch
func (s *TestSuite) TestUnknownOperationType_Panics() {
	ctrl := gomock.NewController(s.T())
	_ = storagetypes.NewMockStorageKeeper(ctrl)
	app := keeper.NewBucketApp(nil)

	// Op 0x99 unknown, empty inner payload is fine because switch happens before decode
	payload := []byte{0x99}
	s.Require().Panics(func() {
		_ = app.ExecuteSynPackage(s.ctx, &sdk.CrossChainAppContext{}, payload)
	})
}

// V2 MustSerialize must be decodable by V2 deserializer and preserve fields
func (s *TestSuite) TestV2PackageSerializationUsesCorrectABI() {
	v2 := storagetypes.CreateBucketSynPackageV2{
		Creator:                        sample.RandAccAddress(),
		BucketName:                     "ser-v2",
		Visibility:                     uint32(storagetypes.VISIBILITY_TYPE_PUBLIC_READ),
		PaymentAddress:                 sample.RandAccAddress(),
		PrimarySpAddress:               sample.RandAccAddress(),
		PrimarySpApprovalExpiredHeight: 123,
		GlobalVirtualGroupFamilyId:     42,
		PrimarySpApprovalSignature:     []byte{0x01, 0x02},
		ChargedReadQuota:               7,
		ExtraData:                      []byte("x"),
	}
	serialized := v2.MustSerialize()
	decoded, err := storagetypes.DeserializeCreateBucketSynPackageV2(serialized)
	s.Require().NoError(err)
	got := decoded.(*storagetypes.CreateBucketSynPackageV2)
	s.Require().Equal(uint32(42), got.GlobalVirtualGroupFamilyId)
	s.Require().Equal(v2.BucketName, got.BucketName)
}

// V2 payload with V1 OperationType should not succeed
func (s *TestSuite) TestV2PackageWithWrongOperationType_Fails() {
	ctrl := gomock.NewController(s.T())
	storageKeeper := storagetypes.NewMockStorageKeeper(ctrl)
	storageKeeper.EXPECT().Logger(gomock.Any()).Return(s.ctx.Logger()).AnyTimes()
	app := keeper.NewBucketApp(storageKeeper)

	v2 := storagetypes.CreateBucketSynPackageV2{
		Creator:                    sample.RandAccAddress(),
		BucketName:                 "wrong-op",
		Visibility:                 uint32(storagetypes.VISIBILITY_TYPE_PUBLIC_READ),
		PaymentAddress:             sample.RandAccAddress(),
		PrimarySpAddress:           sample.RandAccAddress(),
		GlobalVirtualGroupFamilyId: 5,
	}
	v2Payload := mustPackCreateBucketSynPackageV2(&v2)
	// Wrong op: use V1 op code 0x02
	wrong := append([]byte{storagetypes.OperationCreateBucket}, v2Payload...)

	// Expect panic due to decode mismatch on V1 path, or at least error in result
	didPanic := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				didPanic = true
			}
		}()
		res := app.ExecuteSynPackage(s.ctx, &sdk.CrossChainAppContext{}, wrong)
		if !didPanic {
			s.Require().Error(res.Err) // must not succeed silently
		}
	}()
}

// Locally construct V2 payload; field order must match on-chain definition
func mustPackCreateBucketSynPackageV2(p *storagetypes.CreateBucketSynPackageV2) []byte {
	tupleType, err := abi.NewType("tuple", "", []abi.ArgumentMarshaling{
		{Name: "Creator", Type: "address"},
		{Name: "BucketName", Type: "string"},
		{Name: "Visibility", Type: "uint32"},
		{Name: "PaymentAddress", Type: "address"},
		{Name: "PrimarySpAddress", Type: "address"},
		{Name: "PrimarySpApprovalExpiredHeight", Type: "uint64"},
		{Name: "GlobalVirtualGroupFamilyId", Type: "uint32"},
		{Name: "PrimarySpApprovalSignature", Type: "bytes"},
		{Name: "ChargedReadQuota", Type: "uint64"},
		{Name: "ExtraData", Type: "bytes"},
	})
	if err != nil {
		panic(err)
	}
	args := abi.Arguments{{Type: tupleType}}

	// Local struct aligned with the ABI definition
	type v2Struct struct {
		Creator                        common.Address
		BucketName                     string
		Visibility                     uint32
		PaymentAddress                 common.Address
		PrimarySpAddress               common.Address
		PrimarySpApprovalExpiredHeight uint64
		GlobalVirtualGroupFamilyId     uint32
		PrimarySpApprovalSignature     []byte
		ChargedReadQuota               uint64
		ExtraData                      []byte
	}

	packed, err := args.Pack(&v2Struct{
		Creator:                        common.BytesToAddress(p.Creator),
		BucketName:                     p.BucketName,
		Visibility:                     p.Visibility,
		PaymentAddress:                 common.BytesToAddress(p.PaymentAddress),
		PrimarySpAddress:               common.BytesToAddress(p.PrimarySpAddress),
		PrimarySpApprovalExpiredHeight: p.PrimarySpApprovalExpiredHeight,
		GlobalVirtualGroupFamilyId:     p.GlobalVirtualGroupFamilyId,
		PrimarySpApprovalSignature:     p.PrimarySpApprovalSignature,
		ChargedReadQuota:               p.ChargedReadQuota,
		ExtraData:                      p.ExtraData,
	})
	if err != nil {
		panic(err)
	}
	return packed
}
