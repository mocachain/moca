package keeper_test

import (
	"fmt"

	sdkmath "cosmossdk.io/math"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/storage/types"
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

func (s *TestSuite) TestSetBucketFlowRateLimit() {
	operatorAddress := sample.RandAccAddress()
	bucketOwner := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))

	// case 1: operator is not owner of payment account
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(false)

	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operatorAddress, bucketOwner, paymentAccount, bucketName, sdkmath.NewInt(1))
	s.Require().ErrorContains(err, "not payment account owner")

	// case 2: bucket is not found
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	err = s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operatorAddress, bucketOwner, paymentAccount, bucketName, sdkmath.NewInt(1))
	s.Require().NoError(err)

	bucketInfo := &types.BucketInfo{
		Owner:            bucketOwner.String(),
		BucketName:       bucketName,
		Id:               sdkmath.NewUint(1),
		PaymentAddress:   paymentAccount.String(),
		ChargedReadQuota: 0,
		BucketStatus:     types.BUCKET_STATUS_CREATED,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	// case 3: different bucket owner
	err = s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operatorAddress, sample.RandAccAddress(), paymentAccount, bucketName, sdkmath.NewInt(1))
	s.Require().NoError(err)

	// case 4: bucket does not use the payment account
	err = s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operatorAddress, bucketOwner, sample.RandAccAddress(), bucketName, sdkmath.NewInt(1))
	s.Require().NoError(err)
}

func (s *TestSuite) TestSetZeroBucketFlowRateLimit() {
	operatorAddress := sample.RandAccAddress()
	bucketOwner := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))

	bucketInfo := &types.BucketInfo{
		Owner:            bucketOwner.String(),
		BucketName:       bucketName,
		Id:               sdkmath.NewUint(1),
		PaymentAddress:   paymentAccount.String(),
		ChargedReadQuota: 100,
	}
	prepareReadStoreBill(s, bucketInfo)

	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operatorAddress, bucketOwner, paymentAccount, bucketName, sdkmath.NewInt(0))
	s.Require().NoError(err)
}

func (s *TestSuite) TestSetFlowRateLimit_NotLimited() {
	operatorAddress := sample.RandAccAddress()
	bucketOwner := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))

	bucketInfo := &types.BucketInfo{
		Owner:            bucketOwner.String(),
		BucketName:       bucketName,
		Id:               sdkmath.NewUint(1),
		PaymentAddress:   paymentAccount.String(),
		ChargedReadQuota: 100,
	}
	prepareReadStoreBill(s, bucketInfo)

	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	internalBucketInfo := s.storageKeeper.MustGetInternalBucketInfo(s.ctx, bucketInfo.Id)
	bill, err := s.storageKeeper.GetBucketReadStoreBill(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().NoError(err)
	totalOutFlowRate := getTotalOutFlowRate(bill.Flows)

	// case 1: rate limit does not exist before and is larger than total out flow rate
	err = s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operatorAddress, bucketOwner, paymentAccount, bucketName, totalOutFlowRate.Add(sdkmath.NewInt(1)))
	s.Require().NoError(err)

	isRateLimited := s.storageKeeper.IsBucketRateLimited(s.ctx, bucketName)
	s.Require().False(isRateLimited)

	// case 2: rate limit exists before and is equal to the previous flow rate limit
	err = s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operatorAddress, bucketOwner, paymentAccount, bucketName, totalOutFlowRate.Add(sdkmath.NewInt(1)))
	s.Require().NoError(err)

	isRateLimited = s.storageKeeper.IsBucketRateLimited(s.ctx, bucketName)
	s.Require().False(isRateLimited)

	// case 3: rate limit exists before and larger than the previous flow rate limit
	err = s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operatorAddress, bucketOwner, paymentAccount, bucketName, totalOutFlowRate.Add(sdkmath.NewInt(2)))
	s.Require().NoError(err)

	isRateLimited = s.storageKeeper.IsBucketRateLimited(s.ctx, bucketName)
	s.Require().False(isRateLimited)
}

func (s *TestSuite) TestSetBucketFlowRateLimit_Limited() {
	operatorAddress := sample.RandAccAddress()
	bucketOwner := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))

	bucketInfo := &types.BucketInfo{
		Owner:            bucketOwner.String(),
		BucketName:       bucketName,
		Id:               sdkmath.NewUint(1),
		PaymentAddress:   paymentAccount.String(),
		ChargedReadQuota: 100,
	}
	prepareReadStoreBill(s, bucketInfo)

	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	internalBucketInfo := s.storageKeeper.MustGetInternalBucketInfo(s.ctx, bucketInfo.Id)
	bill, err := s.storageKeeper.GetBucketReadStoreBill(s.ctx, bucketInfo, internalBucketInfo)
	s.Require().NoError(err)
	totalOutFlowRate := getTotalOutFlowRate(bill.Flows)

	// case 1: rate limit does not exist before and is less than total out flow rate
	err = s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operatorAddress, bucketOwner, paymentAccount, bucketName, totalOutFlowRate.Sub(sdkmath.NewInt(1)))
	s.Require().NoError(err)

	isRateLimited := s.storageKeeper.IsBucketRateLimited(s.ctx, bucketName)
	s.Require().True(isRateLimited)

	// case 2: rate limit exists before and is equal to the previous flow rate limit
	err = s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operatorAddress, bucketOwner, paymentAccount, bucketName, totalOutFlowRate.Sub(sdkmath.NewInt(1)))
	s.Require().NoError(err)

	isRateLimited = s.storageKeeper.IsBucketRateLimited(s.ctx, bucketName)
	s.Require().True(isRateLimited)

	// case 3: bucket is rate limited and the new rate limit is less than the total out flow rate
	err = s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operatorAddress, bucketOwner, paymentAccount, bucketName, totalOutFlowRate.Sub(sdkmath.NewInt(2)))
	s.Require().NoError(err)

	isRateLimited = s.storageKeeper.IsBucketRateLimited(s.ctx, bucketName)
	s.Require().True(isRateLimited)

	// case 4: bucket is rate limited and the new rate limit is larger than the total out flow rate
	err = s.storageKeeper.SetBucketFlowRateLimit(s.ctx, operatorAddress, bucketOwner, paymentAccount, bucketName, totalOutFlowRate.Add(sdkmath.NewInt(1)))
	s.Require().NoError(err)

	// the bucket should be no longer rate limited
	isRateLimited = s.storageKeeper.IsBucketRateLimited(s.ctx, bucketName)
	s.Require().False(isRateLimited)
}

func getTotalOutFlowRate(flows []paymenttypes.OutFlow) sdkmath.Int {
	totalFlowRate := sdkmath.ZeroInt()
	for _, flow := range flows {
		totalFlowRate = totalFlowRate.Add(flow.Rate)
	}
	return totalFlowRate
}

func prepareReadStoreBill(s *TestSuite, bucketInfo *types.BucketInfo) {
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{
		Id:                    1,
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return(gvgFamily, true).AnyTimes()

	bucketInfo.GlobalVirtualGroupFamilyId = gvgFamily.Id

	primarySp := &sptypes.StorageProvider{
		Status:          sptypes.STATUS_IN_SERVICE,
		Id:              100,
		OperatorAddress: sample.RandAccAddress().String(),
		FundingAddress:  sample.RandAccAddress().String(),
	}
	s.spKeeper.EXPECT().GetStorageProvider(gomock.Any(), gomock.Eq(primarySp.Id)).
		Return(primarySp, true).AnyTimes()

	price := sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(100),
		PrimaryStorePrice:   sdkmath.LegacyNewDec(1000),
		SecondaryStorePrice: sdkmath.LegacyNewDec(500),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).
		Return(price, nil).AnyTimes()
	params := paymenttypes.DefaultParams()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(params.VersionedParams, nil).AnyTimes()

	// none empty bucket
	lvg1 := &types.LocalVirtualGroup{
		Id:                   1,
		TotalChargeSize:      100,
		GlobalVirtualGroupId: 1,
	}
	lvg2 := &types.LocalVirtualGroup{
		Id:                   2,
		TotalChargeSize:      200,
		GlobalVirtualGroupId: 2,
	}
	internalBucketInfo := &types.InternalBucketInfo{
		TotalChargeSize: 300,
		LocalVirtualGroups: []*types.LocalVirtualGroup{
			lvg1, lvg2,
		},
	}

	gvg1 := &virtualgroupmoduletypes.GlobalVirtualGroup{
		Id:                    1,
		PrimarySpId:           primarySp.Id,
		SecondarySpIds:        []uint32{101, 102, 103, 104, 105, 106},
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	}
	gvg2 := &virtualgroupmoduletypes.GlobalVirtualGroup{
		Id:                    2,
		PrimarySpId:           primarySp.Id,
		SecondarySpIds:        []uint32{201, 202, 203, 204, 205, 206},
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	}
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gvg1.Id).
		Return(gvg1, true).AnyTimes()
	s.virtualGroupKeeper.EXPECT().GetGVG(gomock.Any(), gvg2.Id).
		Return(gvg2, true).AnyTimes()

	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, internalBucketInfo)
}

func (s *TestSuite) TestChargeBucketReadFee() {
}

func (s *TestSuite) TestSetBucketFlowRateLimit_SetFlowRateLimitError() {
	ownerAddr := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))

	bucketInfo := &types.BucketInfo{
		Owner: ownerAddr.String(), PaymentAddress: paymentAccount.String(), BucketName: bucketName,
		Id: sdkmath.NewUint(1), ChargedReadQuota: 100, GlobalVirtualGroupFamilyId: 999,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroupFamily)(nil), false).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()

	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, ownerAddr, ownerAddr, paymentAccount, bucketName, sdkmath.NewInt(1))
	s.Require().ErrorContains(err, "get bucket bill failed")
}

func (s *TestSuite) TestSetBucketFlowRateLimit_UnchargeBillError() {
	ownerAddr := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))
	bucketInfo := &types.BucketInfo{
		Owner: ownerAddr.String(), PaymentAddress: paymentAccount.String(), BucketName: bucketName,
		Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 1, ChargedReadQuota: 100,
	}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	// The bucket's own bill lookup (line 136) must succeed so the "newly over limit" branch is
	// taken; only the second lookup, inside unChargeBucketReadStoreFee, fails.
	gomock.InOrder(
		s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil),
		s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(sptypes.GlobalSpStorePrice{}, fmt.Errorf("boom")),
	)
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, ownerAddr, ownerAddr, paymentAccount, bucketName, sdkmath.NewInt(0))
	s.Require().ErrorContains(err, "get bucket bill failed")
}

func (s *TestSuite) TestSetBucketFlowRateLimit_UnchargeApplyError() {
	ownerAddr := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	bucketName := string(sample.RandStr(10))
	bucketInfo := &types.BucketInfo{
		Owner: ownerAddr.String(), PaymentAddress: paymentAccount.String(), BucketName: bucketName,
		Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 1, ChargedReadQuota: 100,
	}
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{Id: 1, VirtualPaymentAddress: sample.RandAccAddress().String()}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()
	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyNewDec(1), PrimaryStorePrice: sdkmath.LegacyZeroDec(), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	s.paymentKeeper.EXPECT().ApplyUserFlowsList(gomock.Any(), gomock.Any()).Return(fmt.Errorf("boom"))
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, ownerAddr, ownerAddr, paymentAccount, bucketName, sdkmath.NewInt(0))
	s.Require().ErrorContains(err, "apply user flows list failed")
}

func (s *TestSuite) TestGetBucketExtraInfo_MalformedPaymentAddress() {
	bucketInfo := &types.BucketInfo{PaymentAddress: "", Owner: sample.RandAccAddress().String(), BucketName: "extrainfo-malformed-bucket"}

	_, err := s.storageKeeper.GetBucketExtraInfo(s.ctx, bucketInfo)
	s.Require().Error(err)
}

func (s *TestSuite) TestGetBucketExtraInfo_NotFoundRateLimit() {
	bucketInfo := &types.BucketInfo{
		PaymentAddress: sample.RandAccAddress().String(), Owner: sample.RandAccAddress().String(),
		BucketName: "extrainfo-notfound-bucket", Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 1, ChargedReadQuota: 0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	extraInfo, err := s.storageKeeper.GetBucketExtraInfo(s.ctx, bucketInfo)
	s.Require().NoError(err)
	s.Require().Equal(sdkmath.NewInt(-1), extraInfo.FlowRateLimit, "no stored rate limit must report -1")
	s.Require().False(extraInfo.IsRateLimited)
	s.Require().True(extraInfo.CurrentFlowRate.IsZero(), "an empty bucket has zero current flow rate")
}

func (s *TestSuite) TestGetBucketExtraInfo_FoundRateLimit() {
	ownerAddr := sample.RandAccAddress()
	paymentAccount := sample.RandAccAddress()
	bucketName := "extrainfo-found-bucket"
	bucketInfo := &types.BucketInfo{
		PaymentAddress: paymentAccount.String(), Owner: ownerAddr.String(),
		BucketName: bucketName, Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 1, ChargedReadQuota: 0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	s.paymentKeeper.EXPECT().IsPaymentAccountOwner(gomock.Any(), gomock.Any(), gomock.Any()).Return(true).AnyTimes()
	err := s.storageKeeper.SetBucketFlowRateLimit(s.ctx, ownerAddr, ownerAddr, paymentAccount, bucketName, sdkmath.NewInt(42))
	s.Require().NoError(err)

	extraInfo, err := s.storageKeeper.GetBucketExtraInfo(s.ctx, bucketInfo)
	s.Require().NoError(err)
	s.Require().Equal(sdkmath.NewInt(42), extraInfo.FlowRateLimit)
}

func (s *TestSuite) TestGetBucketExtraInfo_BillError() {
	bucketInfo := &types.BucketInfo{
		PaymentAddress: sample.RandAccAddress().String(), Owner: sample.RandAccAddress().String(),
		BucketName: "extrainfo-billerror-bucket", Id: sdkmath.NewUint(1), GlobalVirtualGroupFamilyId: 999, ChargedReadQuota: 100,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).
		Return((*virtualgroupmoduletypes.GlobalVirtualGroupFamily)(nil), false).AnyTimes()

	_, err := s.storageKeeper.GetBucketExtraInfo(s.ctx, bucketInfo)
	s.Require().ErrorContains(err, "get bucket bill failed")
}
