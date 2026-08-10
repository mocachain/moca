package keeper_test

import (
	sdkmath "cosmossdk.io/math"
	"go.uber.org/mock/gomock"

	"github.com/mocachain/moca/v2/testutil/sample"
	types2 "github.com/mocachain/moca/v2/types"
	permtypes "github.com/mocachain/moca/v2/x/permission/types"
	"github.com/mocachain/moca/v2/x/storage/types"
)

// TestPutPolicy_RunsValidateRuntime is a regression test for MOCA-963/964
// (): MsgPutPolicy.ValidateRuntime existed but was never invoked
// by any caller, so every check it performs — bucket-level actions, Resources
// on a non-bucket resource, LimitSize without CreateObject — was dead. A
// bucket-level statement naming a group-only action clears ValidateBasic (which
// never consults BucketAllowedActionsAfterPampas) and must now be rejected.
//
// msgServer.PutPolicy is the single implementation shared by both write paths:
// the native Cosmos tx path (x/storage/module.go registers
// keeper.NewMsgServerImpl(k) as the module's MsgServer) and the EVM storage
// precompile's PutPolicy (precompiles/storage/tx.go calls
// p.storageMsgServer.PutPolicy, and app.go wires that field to the very same
// keeper.NewMsgServerImpl(app.StorageKeeper)). Fixing it here closes both
// MOCA-963 and MOCA-964 in one place.
func (s *TestSuite) TestPutPolicy_RunsValidateRuntime() {
	operator := sample.RandAccAddress()
	bucketName := "putpolicy-runtime-bucket"

	bucketInfo := &types.BucketInfo{
		Owner:            operator.String(),
		BucketName:       bucketName,
		Id:               sdkmath.NewUint(1),
		PaymentAddress:   sample.RandAccAddress().String(),
		ChargedReadQuota: 100,
		BucketStatus:     types.BUCKET_STATUS_CREATED,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	principal := sample.RandAccAddress()
	msg := types.NewMsgPutPolicy(
		operator,
		types2.NewBucketGRN(bucketName).String(),
		permtypes.NewPrincipalWithAccount(principal),
		[]*permtypes.Statement{
			{
				Effect: permtypes.EFFECT_ALLOW,
				// A group-only action on a bucket resource: ValidateBasic does not
				// check the bucket action map, ValidateRuntime does.
				Actions:   []permtypes.ActionType{permtypes.ACTION_UPDATE_GROUP_MEMBER},
				Resources: []string{"grn:o::" + bucketName + "/obj"},
			},
		},
		nil,
	)
	s.Require().NoError(msg.ValidateBasic(), "the statement must clear ValidateBasic for this test to be meaningful")

	_, err := s.msgServer.PutPolicy(s.ctx, msg)
	s.Require().Error(err, "PutPolicy must run MsgPutPolicy.ValidateRuntime")
	s.Require().ErrorIs(err, permtypes.ErrInvalidStatement)
}

// TestPutPolicy_AcceptsRegexpMetacharacterInObjectName pins the accept path so
// the previous test cannot pass merely by rejecting every PutPolicy call, and
// pins that a Resources entry which is a legal object name but not a legal Go
// regexp stays storable: Resources are wildcard patterns, not regexps.
func (s *TestSuite) TestPutPolicy_AcceptsRegexpMetacharacterInObjectName() {
	operator := sample.RandAccAddress()
	bucketName := "putpolicy-metachar-bucket"

	bucketInfo := &types.BucketInfo{
		Owner:            operator.String(),
		BucketName:       bucketName,
		Id:               sdkmath.NewUint(1),
		PaymentAddress:   sample.RandAccAddress().String(),
		ChargedReadQuota: 100,
		BucketStatus:     types.BUCKET_STATUS_CREATED,
	}
	s.storageKeeper.StoreBucketInfo(s.ctx, bucketInfo)

	principal := sample.RandAccAddress()
	msg := types.NewMsgPutPolicy(
		operator,
		types2.NewBucketGRN(bucketName).String(),
		permtypes.NewPrincipalWithAccount(principal),
		[]*permtypes.Statement{
			{
				Effect:    permtypes.EFFECT_ALLOW,
				Actions:   []permtypes.ActionType{permtypes.ACTION_GET_OBJECT},
				Resources: []string{"grn:o::" + bucketName + "/obj["},
			},
		},
		nil,
	)
	s.Require().NoError(msg.ValidateBasic())

	s.permissionKeeper.EXPECT().PutPolicy(gomock.Any(), gomock.Any()).Return(sdkmath.OneUint(), nil)

	_, err := s.msgServer.PutPolicy(s.ctx, msg)
	s.Require().NoError(err, "a legal object name that is not a legal regexp must remain storable")
}
