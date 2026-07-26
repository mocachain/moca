package codec

import (
	"github.com/cosmos/cosmos-sdk/types/module"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"

	"github.com/mocachain/moca/v2/encoding"
)

// TestEncodingConfig re-exports the SDK type so callers that previously used
// `moduletestutil.TestEncodingConfig` keep compiling unchanged.
type TestEncodingConfig = sdktestutil.TestEncodingConfig

// MakeTestEncodingConfig builds a TestEncodingConfig backed by moca's
// production codec wiring and additionally registers any module interfaces
// requested by the caller.
//
// The upstream sdk helper (moduletestutil.MakeTestEncodingConfig) builds an
// InterfaceRegistry from an empty CodecOptions{}; on cosmos-sdk v0.53 that
// path errors with "address codec is required" because the signing context
// has no address codec configured. Routing through encoding.MakeConfig() —
// which threads moca's MultiPrefixBech32 codecs into signing.Options —
// sidesteps that.
func MakeTestEncodingConfig(modules ...module.AppModuleBasic) TestEncodingConfig {
	cfg := encoding.MakeConfig()
	for _, m := range modules {
		m.RegisterInterfaces(cfg.InterfaceRegistry)
		m.RegisterLegacyAminoCodec(cfg.Amino)
	}
	return cfg
}
