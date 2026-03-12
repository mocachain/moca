package keeper

import (
	"github.com/evmos/evmos/v12/x/storage/types"
)

func RegisterCrossApps(keeper Keeper) {
	bucketApp := NewBucketApp(keeper)
	err := keeper.crossChainKeeper.RegisterChannel(types.BucketChannel, types.BucketChannelID, bucketApp)
	if err != nil {
		panic(err)
	}

	objectApp := NewObjectApp(keeper)
	err = keeper.crossChainKeeper.RegisterChannel(types.ObjectChannel, types.ObjectChannelID, objectApp)
	if err != nil {
		panic(err)
	}

	groupApp := NewGroupApp(keeper)
	err = keeper.crossChainKeeper.RegisterChannel(types.GroupChannel, types.GroupChannelID, groupApp)
	if err != nil {
		panic(err)
	}

	mocasbtApp := NewMocaSBTApp(keeper)
	err = keeper.crossChainKeeper.RegisterChannel(types.MocaSBTChannel, types.MocaSBTChannelId, mocasbtApp)
	if err != nil {
		panic(err)
	}

	mocavcApp := NewMocaVCApp(keeper)
	err = keeper.crossChainKeeper.RegisterChannel(types.MocaVCChannel, types.MocaVCChannelId, mocavcApp)
	if err != nil {
		panic(err)
	}

	permissionApp := NewPermissionApp(keeper, keeper.permKeeper)
	err = keeper.crossChainKeeper.RegisterChannel(types.PermissionChannel, types.PermissionChannelID, permissionApp)
	if err != nil {
		panic(err)
	}
}
