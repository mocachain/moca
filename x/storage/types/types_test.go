package types_test

import (
	"testing"

	"cosmossdk.io/math"
	"github.com/stretchr/testify/require"

	"github.com/mocachain/moca/v2/x/storage/types"
)

func TestBucketInfo_ToNFTMetadata(t *testing.T) {
	info := &types.BucketInfo{
		Owner:      "0xowner",
		BucketName: "mybucket",
	}
	meta := info.ToNFTMetadata()
	require.Equal(t, "mybucket", meta.BucketName)
	require.Contains(t, meta.Attributes, types.Trait{TraitType: "Owner", Value: "0xowner"},
		"non-omitted fields must show up as NFT traits")
}

func TestBucketInfo_CheckBucketStatus(t *testing.T) {
	tests := []struct {
		name   string
		status types.BucketStatus
		err    error
	}{
		{"created is fine", types.BUCKET_STATUS_CREATED, nil},
		{"discontinued is rejected", types.BUCKET_STATUS_DISCONTINUED, types.ErrBucketDiscontinued},
		{"migrating is rejected", types.BUCKET_STATUS_MIGRATING, types.ErrBucketMigrating},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &types.BucketInfo{BucketStatus: tt.status}
			err := info.CheckBucketStatus()
			if tt.err == nil {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, tt.err)
		})
	}
}

func TestObjectInfo_ToNFTMetadata(t *testing.T) {
	info := &types.ObjectInfo{
		ObjectName:   "myobject",
		ObjectStatus: types.OBJECT_STATUS_SEALED,
		Checksums:    [][]byte{[]byte("checksum")},
	}
	meta := info.ToNFTMetadata()
	require.Equal(t, "myobject", meta.ObjectName)

	var traitNames []string
	for _, tr := range meta.Attributes {
		traitNames = append(traitNames, tr.TraitType)
	}
	require.Contains(t, traitNames, "ObjectStatus", "a regular field must appear as a trait")
	require.NotContains(t, traitNames, "Checksums", "the traits:omit field must be skipped")
}

func TestGroupInfo_ToNFTMetadata(t *testing.T) {
	info := &types.GroupInfo{GroupName: "mygroup"}
	meta := info.ToNFTMetadata()
	require.Equal(t, "mygroup", meta.GroupName)
	require.Contains(t, meta.Attributes, types.Trait{TraitType: "GroupName", Value: "mygroup"})
}

func TestObjectInfo_GetLatestUpdatedTime(t *testing.T) {
	tests := []struct {
		name      string
		createAt  int64
		updatedAt int64
		want      int64
	}{
		{"never updated falls back to create time", 100, 0, 100},
		{"updated time takes precedence", 100, 200, 200},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &types.ObjectInfo{CreateAt: tt.createAt, UpdatedAt: tt.updatedAt}
			require.Equal(t, tt.want, info.GetLatestUpdatedTime())
		})
	}
}

func idList(ids ...uint64) *types.Ids {
	out := make([]types.Uint, len(ids))
	for i, id := range ids {
		out[i] = math.NewUint(id)
	}
	return &types.Ids{Id: out}
}

func TestDeleteInfo_IsEmpty(t *testing.T) {
	var nilInfo *types.DeleteInfo
	require.True(t, nilInfo.IsEmpty(), "a nil DeleteInfo must be treated as empty")

	tests := []struct {
		name string
		info *types.DeleteInfo
		want bool
	}{
		{"all nil id sets", &types.DeleteInfo{}, true},
		{"all empty id sets", &types.DeleteInfo{BucketIds: idList(), ObjectIds: idList(), GroupIds: idList()}, true},
		{"only bucket ids populated", &types.DeleteInfo{BucketIds: idList(1)}, false},
		{"only object ids populated", &types.DeleteInfo{ObjectIds: idList(2)}, false},
		{"only group ids populated", &types.DeleteInfo{GroupIds: idList(3)}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, tt.info.IsEmpty())
		})
	}
}

func TestInternalBucketInfo_LVGLifecycle(t *testing.T) {
	b := &types.InternalBucketInfo{}

	_, found := b.GetLVGByGVGID(1)
	require.False(t, found, "no LVG exists yet")
	require.Equal(t, uint32(0), b.GetMaxLVGID(), "max id on an empty list is zero")

	lvg1 := &types.LocalVirtualGroup{Id: 1, GlobalVirtualGroupId: 10}
	b.AppendLVG(lvg1)
	require.Equal(t, uint32(1), b.GetMaxLVGID())

	lvg2 := &types.LocalVirtualGroup{Id: 2, GlobalVirtualGroupId: 20}
	b.AppendLVG(lvg2)
	require.Equal(t, uint32(2), b.GetMaxLVGID())

	require.Panics(t, func() {
		b.AppendLVG(&types.LocalVirtualGroup{Id: 2, GlobalVirtualGroupId: 30})
	}, "appending a non-increasing id must panic")
	require.Panics(t, func() {
		b.AppendLVG(&types.LocalVirtualGroup{Id: 1, GlobalVirtualGroupId: 30})
	}, "appending an id smaller than the last one must panic")

	byGVG, found := b.GetLVGByGVGID(20)
	require.True(t, found)
	require.Same(t, lvg2, byGVG)
	_, found = b.GetLVGByGVGID(99)
	require.False(t, found)

	byID, found := b.GetLVG(1)
	require.True(t, found)
	require.Same(t, lvg1, byID)
	_, found = b.GetLVG(99)
	require.False(t, found)

	require.Same(t, lvg1, b.MustGetLVG(1))
	require.Panics(t, func() { b.MustGetLVG(99) }, "MustGetLVG must panic when the id is absent")

	b.DeleteLVG(99)
	require.Len(t, b.LocalVirtualGroups, 2, "deleting an absent id is a no-op")

	b.DeleteLVG(1)
	require.Len(t, b.LocalVirtualGroups, 1)
	_, found = b.GetLVG(1)
	require.False(t, found, "the deleted lvg must be gone")
	remaining, found := b.GetLVG(2)
	require.True(t, found)
	require.Same(t, lvg2, remaining)
}
