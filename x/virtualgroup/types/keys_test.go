package types

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/require"
)

// encodeSeq mirrors internal/sequence's uint32 encoding (big-endian, 4
// bytes) so the expected key bytes below are computed independently of the
// production helper being tested.
func encodeSeq(id uint32) []byte {
	bz := make([]byte, 4)
	binary.BigEndian.PutUint32(bz, id)
	return bz
}

func TestGetKeys(t *testing.T) {
	tests := []struct {
		name   string
		fn     func(uint32) []byte
		prefix []byte
		id     uint32
	}{
		{"gvg", GetGVGKey, GVGKey, 7},
		{"gvg family", GetGVGFamilyKey, GVGFamilyKey, 7},
		{"gvg family statistics within sp", GetGVGFamilyStatisticsWithinSPKey, GVGFamilyStatisticsWithinSPKey, 3},
		{"gvg statistics within sp", GetGVGStatisticsWithinSPKey, GVGStatisticsWithinSPKey, 3},
		{"swap out family", GetSwapOutFamilyKey, SwapOutFamilyKey, 11},
		{"swap out gvg", GetSwapOutGVGKey, SwapOutGVGKey, 11},
		{"swap in family", GetSwapInFamilyKey, SwapInFamilyKey, 5},
		{"swap in gvg", GetSwapInGVGKey, SwapInGVGKey, 5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			want := append(append([]byte{}, tt.prefix...), encodeSeq(tt.id)...)
			require.Equal(t, want, tt.fn(tt.id))
		})
	}
}
