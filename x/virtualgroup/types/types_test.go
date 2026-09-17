package types

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGlobalVirtualGroupFamily_AppendGVG(t *testing.T) {
	f := &GlobalVirtualGroupFamily{}
	f.AppendGVG(1)
	f.AppendGVG(2)
	require.Equal(t, []uint32{1, 2}, f.GlobalVirtualGroupIds)
}

func TestGlobalVirtualGroupFamily_Contains(t *testing.T) {
	f := &GlobalVirtualGroupFamily{GlobalVirtualGroupIds: []uint32{1, 2, 3}}
	require.True(t, f.Contains(2))
	require.False(t, f.Contains(4))
}

func TestGlobalVirtualGroupFamily_RemoveGVG(t *testing.T) {
	tests := []struct {
		name    string
		ids     []uint32
		remove  uint32
		wantIDs []uint32
		wantErr error
	}{
		{
			name:    "found",
			ids:     []uint32{1, 2, 3},
			remove:  2,
			wantIDs: []uint32{1, 3},
		},
		{
			name:    "not found",
			ids:     []uint32{1, 2, 3},
			remove:  9,
			wantIDs: []uint32{1, 2, 3},
			wantErr: ErrGVGNotExist,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &GlobalVirtualGroupFamily{GlobalVirtualGroupIds: append([]uint32{}, tt.ids...)}
			err := f.RemoveGVG(tt.remove)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				require.Equal(t, tt.wantIDs, f.GlobalVirtualGroupIds)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantIDs, f.GlobalVirtualGroupIds)
		})
	}
}

func TestGlobalVirtualGroupFamily_MustRemoveGVG(t *testing.T) {
	f := &GlobalVirtualGroupFamily{GlobalVirtualGroupIds: []uint32{1, 2}}
	require.NotPanics(t, func() { f.MustRemoveGVG(1) })
	require.Equal(t, []uint32{2}, f.GlobalVirtualGroupIds)

	require.PanicsWithValue(t, fmt.Sprintf("remove gvg from family failed. err: %s", ErrGVGNotExist), func() {
		f.MustRemoveGVG(99)
	})
}

func TestGlobalVirtualGroupsBindingOnBucket_AppendGVGAndLVG(t *testing.T) {
	g := &GlobalVirtualGroupsBindingOnBucket{}
	g.AppendGVGAndLVG(1, 10)
	g.AppendGVGAndLVG(2, 20)
	require.Equal(t, []uint32{1, 2}, g.GlobalVirtualGroupIds)
	require.Equal(t, []uint32{10, 20}, g.LocalVirtualGroupIds)
}

func TestGlobalVirtualGroupsBindingOnBucket_GetLVGIDByGVGID(t *testing.T) {
	g := &GlobalVirtualGroupsBindingOnBucket{
		GlobalVirtualGroupIds: []uint32{1, 2},
		LocalVirtualGroupIds:  []uint32{10, 20},
	}
	require.Equal(t, uint32(20), g.GetLVGIDByGVGID(2))
	require.Equal(t, uint32(0), g.GetLVGIDByGVGID(99))
}

func TestGlobalVirtualGroupsBindingOnBucket_GetGVGIDByLVGID(t *testing.T) {
	g := &GlobalVirtualGroupsBindingOnBucket{
		GlobalVirtualGroupIds: []uint32{1, 2},
		LocalVirtualGroupIds:  []uint32{10, 20},
	}
	require.Equal(t, uint32(2), g.GetGVGIDByLVGID(20))
	require.Equal(t, uint32(0), g.GetGVGIDByLVGID(99))
}
