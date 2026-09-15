package keeper_test

import (
	sdkmath "cosmossdk.io/math"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	paymenttypes "github.com/mocachain/moca/v2/x/payment/types"
	sptypes "github.com/mocachain/moca/v2/x/sp/types"
	"github.com/mocachain/moca/v2/x/storage/types"
	virtualgroupmoduletypes "github.com/mocachain/moca/v2/x/virtualgroup/types"
)

// zeroStreamRecord returns a StreamRecord with every sdkmath.Int field the
// RunPaymentCheck reconciliation loops operate on (LockBalance, NetflowRate,
// FrozenNetflowRate) initialized to a valid zero value. RunPaymentCheck's
// final reverse-check loop unconditionally calls IsPositive/IsNegative on
// all three fields for every stream record, and a zero-value sdkmath.Int
// (nil big.Int) panics on those calls, so callers must start from this
// helper and only override the fields their scenario needs.
func zeroStreamRecord(account string) paymenttypes.StreamRecord {
	return paymenttypes.StreamRecord{
		Account:           account,
		NetflowRate:       sdkmath.ZeroInt(),
		StaticBalance:     sdkmath.ZeroInt(),
		BufferBalance:     sdkmath.ZeroInt(),
		LockBalance:       sdkmath.ZeroInt(),
		FrozenNetflowRate: sdkmath.ZeroInt(),
		Status:            paymenttypes.STREAM_ACCOUNT_STATUS_ACTIVE,
	}
}

// newReadFeeBucket seeds a bucket whose only payment component is a single
// positive read-fee outflow (zero validator tax, no local virtual groups and
// no objects), so it contributes exactly one entry to userFlowRateMap (keyed
// by paymentAddr) and one to receiverFlowRateMap (keyed by the returned
// address), and nothing to lockBalanceMap. That keeps each RunPaymentCheck
// test's induced mismatch isolated to a single map entry, sidestepping the
// map-iteration-order non-determinism when more than one mismatch exists.
func newReadFeeBucket(s *TestSuite, paymentAddr, bucketName string) (receiverAddr string, rate sdkmath.Int) {
	gvgFamily := &virtualgroupmoduletypes.GlobalVirtualGroupFamily{
		Id:                    1,
		VirtualPaymentAddress: sample.RandAccAddress().String(),
	}
	s.virtualGroupKeeper.EXPECT().GetGVGFamily(gomock.Any(), gomock.Any()).Return(gvgFamily, true).AnyTimes()

	bucketInfo := &types.BucketInfo{
		Owner:                      paymentAddr,
		BucketName:                 bucketName,
		Id:                         sdkmath.NewUint(1),
		PaymentAddress:             paymentAddr,
		GlobalVirtualGroupFamilyId: gvgFamily.Id,
		ChargedReadQuota:           100,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	price := sptypes.GlobalSpStorePrice{
		ReadPrice:           sdkmath.LegacyNewDec(1),
		PrimaryStorePrice:   sdkmath.LegacyZeroDec(),
		SecondaryStorePrice: sdkmath.LegacyZeroDec(),
	}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()

	return gvgFamily.VirtualPaymentAddress, sdkmath.NewInt(100)
}

// TestRunPaymentCheck_LockBalanceMismatch covers the "lock balance not equal"
// branch of the expected->actual lock balance comparison.
func (s *TestSuite) TestRunPaymentCheck_LockBalanceMismatch() {
	paymentAddr := sample.RandAccAddress().String()
	bucketName := "lockbalance-mismatch-bucket"
	priceTime := s.ctx.BlockTime().Unix() + 1

	bucketInfo := &types.BucketInfo{
		Owner: paymentAddr, BucketName: bucketName, Id: sdkmath.NewUint(1),
		PaymentAddress: paymentAddr, GlobalVirtualGroupFamilyId: 1, ChargedReadQuota: 0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	objectInfo := &types.ObjectInfo{
		Id: sdkmath.NewUint(1), Owner: paymentAddr, BucketName: bucketName, ObjectName: "lockbalance-mismatch-object",
		PayloadSize: 1024, ObjectStatus: types.OBJECT_STATUS_CREATED, CreateAt: priceTime,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, objectInfo)

	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ReserveTime: 100, ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()

	expectedLockBalance, _, err := s.storageKeeper.GetObjectLockFee(s.ctx, objectInfo.GetLatestUpdatedTime(), objectInfo.PayloadSize)
	s.Require().NoError(err)

	record := zeroStreamRecord(paymentAddr)
	record.LockBalance = expectedLockBalance.AddRaw(1) // deliberately wrong
	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).Return([]paymenttypes.StreamRecord{record}).AnyTimes()

	err = s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().ErrorContains(err, "lock balance not equal")
}

// TestRunPaymentCheck_LockBalanceStreamRecordNotFound covers the "stream
// record not found" branch of the lock balance comparison.
func (s *TestSuite) TestRunPaymentCheck_LockBalanceStreamRecordNotFound() {
	paymentAddr := sample.RandAccAddress().String()
	bucketName := "lockbalance-notfound-bucket"
	priceTime := s.ctx.BlockTime().Unix() + 1

	bucketInfo := &types.BucketInfo{
		Owner: paymentAddr, BucketName: bucketName, Id: sdkmath.NewUint(1),
		PaymentAddress: paymentAddr, GlobalVirtualGroupFamilyId: 1, ChargedReadQuota: 0,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)
	s.storageKeeper.SetInternalBucketInfo(s.ctx, bucketInfo.Id, &types.InternalBucketInfo{})

	objectInfo := &types.ObjectInfo{
		Id: sdkmath.NewUint(1), Owner: paymentAddr, BucketName: bucketName, ObjectName: "lockbalance-notfound-object",
		PayloadSize: 1024, ObjectStatus: types.OBJECT_STATUS_CREATED, CreateAt: priceTime,
	}
	s.storageKeeper.StoreObjectInfo(s.ctx, objectInfo)

	price := sptypes.GlobalSpStorePrice{ReadPrice: sdkmath.LegacyZeroDec(), PrimaryStorePrice: sdkmath.LegacyNewDec(1), SecondaryStorePrice: sdkmath.LegacyZeroDec()}
	s.spKeeper.EXPECT().GetGlobalSpStorePriceByTime(gomock.Any(), gomock.Any()).Return(price, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetVersionedParamsWithTs(gomock.Any(), gomock.Any()).
		Return(paymenttypes.VersionedParams{ReserveTime: 100, ValidatorTaxRate: sdkmath.LegacyZeroDec()}, nil).AnyTimes()
	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).Return([]paymenttypes.StreamRecord{}).AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().ErrorContains(err, "comparing lock balance - stream record not found")
}

// TestRunPaymentCheck_UserFlowRateMismatch covers the "user net flow rate not
// equal" branch.
func (s *TestSuite) TestRunPaymentCheck_UserFlowRateMismatch() {
	paymentAddr := sample.RandAccAddress().String()
	receiverAddr, rate := newReadFeeBucket(s, paymentAddr, "userflow-mismatch-bucket")

	payerRecord := zeroStreamRecord(paymentAddr) // NetflowRate left at zero, wrong: expected -rate
	receiverRecord := zeroStreamRecord(receiverAddr)
	receiverRecord.NetflowRate = rate

	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).
		Return([]paymenttypes.StreamRecord{payerRecord, receiverRecord}).AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().ErrorContains(err, "user net flow rate not equal")
}

// TestRunPaymentCheck_UserFlowRateInvalidStatus covers the "invalid status or
// out flow count" branch: a negative net flow rate with a non-positive
// out-flow count.
func (s *TestSuite) TestRunPaymentCheck_UserFlowRateInvalidStatus() {
	paymentAddr := sample.RandAccAddress().String()
	receiverAddr, rate := newReadFeeBucket(s, paymentAddr, "userflow-invalidstatus-bucket")

	payerRecord := zeroStreamRecord(paymentAddr)
	payerRecord.NetflowRate = rate.Neg() // matches expected, so only the invalid-status branch fires
	payerRecord.OutFlowCount = 0

	receiverRecord := zeroStreamRecord(receiverAddr)
	receiverRecord.NetflowRate = rate

	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).
		Return([]paymenttypes.StreamRecord{payerRecord, receiverRecord}).AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().ErrorContains(err, "user net flow rate invalid status or out flow count")
}

// TestRunPaymentCheck_UserFlowRateStreamRecordNotFound covers the "stream
// record not found" branch of the user net flow rate comparison.
func (s *TestSuite) TestRunPaymentCheck_UserFlowRateStreamRecordNotFound() {
	paymentAddr := sample.RandAccAddress().String()
	receiverAddr, rate := newReadFeeBucket(s, paymentAddr, "userflow-notfound-bucket")

	receiverRecord := zeroStreamRecord(receiverAddr)
	receiverRecord.NetflowRate = rate

	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).
		Return([]paymenttypes.StreamRecord{receiverRecord}).AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().ErrorContains(err, "comparing user net flow rate - stream record not found")
}

// TestRunPaymentCheck_UserFlowRateFrozen is a clean (no-error) pass through
// the frozen-account branch: the payer's stream record is frozen, and its
// frozen out-flow is merged into frozenReceiverFlowRateMap before the
// receiver comparison. Everything is set up to reconcile exactly, so this
// proves the frozen-adjustment logic does not spuriously invent a mismatch.
func (s *TestSuite) TestRunPaymentCheck_UserFlowRateFrozen() {
	paymentAddr := sample.RandAccAddress().String()
	receiverAddr, rate := newReadFeeBucket(s, paymentAddr, "userflow-frozen-bucket")

	payerRecord := zeroStreamRecord(paymentAddr)
	payerRecord.NetflowRate = rate.Neg()
	payerRecord.OutFlowCount = 1
	payerRecord.Status = paymenttypes.STREAM_ACCOUNT_STATUS_FROZEN

	receiverRecord := zeroStreamRecord(receiverAddr)
	receiverRecord.NetflowRate = rate

	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).
		Return([]paymenttypes.StreamRecord{payerRecord, receiverRecord}).AnyTimes()
	s.paymentKeeper.EXPECT().GetOutFlows(gomock.Any(), gomock.Any()).
		Return([]paymenttypes.OutFlow{{ToAddress: receiverAddr, Rate: sdkmath.ZeroInt(), Status: paymenttypes.OUT_FLOW_STATUS_FROZEN}}).
		AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().NoError(err)
}

// TestRunPaymentCheck_ReceiverFlowRateMismatch covers the "receiver net flow
// rate not equal" branch.
func (s *TestSuite) TestRunPaymentCheck_ReceiverFlowRateMismatch() {
	paymentAddr := sample.RandAccAddress().String()
	receiverAddr, rate := newReadFeeBucket(s, paymentAddr, "receiverflow-mismatch-bucket")

	payerRecord := zeroStreamRecord(paymentAddr)
	payerRecord.NetflowRate = rate.Neg()
	payerRecord.OutFlowCount = 1

	receiverRecord := zeroStreamRecord(receiverAddr)
	receiverRecord.NetflowRate = rate.AddRaw(1) // deliberately wrong

	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).
		Return([]paymenttypes.StreamRecord{payerRecord, receiverRecord}).AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().ErrorContains(err, "receiver net flow rate not equal")
}

// TestRunPaymentCheck_ReceiverFlowRateInvalidStatus covers the receiver's
// "invalid status or out flow count" branch (a positive out-flow count).
func (s *TestSuite) TestRunPaymentCheck_ReceiverFlowRateInvalidStatus() {
	paymentAddr := sample.RandAccAddress().String()
	receiverAddr, rate := newReadFeeBucket(s, paymentAddr, "receiverflow-invalidstatus-bucket")

	payerRecord := zeroStreamRecord(paymentAddr)
	payerRecord.NetflowRate = rate.Neg()
	payerRecord.OutFlowCount = 1

	receiverRecord := zeroStreamRecord(receiverAddr)
	receiverRecord.NetflowRate = rate // matches, so only the invalid-status branch fires
	receiverRecord.OutFlowCount = 1

	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).
		Return([]paymenttypes.StreamRecord{payerRecord, receiverRecord}).AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().ErrorContains(err, "receiver net flow rate invalid status or out flow count")
}

// TestRunPaymentCheck_ReceiverFlowRateStreamRecordNotFound covers the
// "stream record not found" branch of the receiver net flow rate comparison.
func (s *TestSuite) TestRunPaymentCheck_ReceiverFlowRateStreamRecordNotFound() {
	paymentAddr := sample.RandAccAddress().String()
	_, rate := newReadFeeBucket(s, paymentAddr, "receiverflow-notfound-bucket")

	payerRecord := zeroStreamRecord(paymentAddr)
	payerRecord.NetflowRate = rate.Neg()
	payerRecord.OutFlowCount = 1

	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).
		Return([]paymenttypes.StreamRecord{payerRecord}).AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().ErrorContains(err, "comparing receiver net flow rate - stream record not found")
}

// The following three tests cover the final "actual -> expected" reverse
// pass, which flags stream records that carry a balance/flow the per-bucket
// scan never produced. No bucket is stored, so lockBalanceMap, userFlowRateMap
// and receiverFlowRateMap are all empty, isolating each orphaned field.

func (s *TestSuite) TestRunPaymentCheck_ReverseLockBalanceOrphan() {
	orphan := zeroStreamRecord(sample.RandAccAddress().String())
	orphan.LockBalance = sdkmath.NewInt(1)

	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).Return([]paymenttypes.StreamRecord{orphan}).AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().ErrorContains(err, "the stream record has lock balance which is not expected")
}

func (s *TestSuite) TestRunPaymentCheck_ReverseUserFlowRateOrphan() {
	orphan := zeroStreamRecord(sample.RandAccAddress().String())
	orphan.NetflowRate = sdkmath.NewInt(-1)

	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).Return([]paymenttypes.StreamRecord{orphan}).AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().ErrorContains(err, "the stream record has negative flow rate which is not expected")
}

func (s *TestSuite) TestRunPaymentCheck_ReverseReceiverFlowRateOrphan() {
	orphan := zeroStreamRecord(sample.RandAccAddress().String())
	orphan.NetflowRate = sdkmath.NewInt(1)

	s.paymentKeeper.EXPECT().GetAllStreamRecord(gomock.Any()).Return([]paymenttypes.StreamRecord{orphan}).AnyTimes()

	err := s.storageKeeper.RunPaymentCheck(s.ctx)
	s.Require().ErrorContains(err, "the stream record has positive flow rate which is not expected")
}
