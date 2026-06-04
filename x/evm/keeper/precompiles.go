package keeper

import (
	"bytes"
	"sort"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"golang.org/x/exp/maps"

	precompileserc20 "github.com/mocachain/moca/v2/x/evm/precompiles/erc20"
)

// Precompiles returns the all precompiled contracts.
func (k Keeper) Precompiles(ctx sdk.Context) ([]common.Address, map[common.Address]vm.PrecompiledContract) {
	addrs := make([]common.Address, 0, len(k.precompiledFunc))
	precompiles := make(map[common.Address]vm.PrecompiledContract, len(k.precompiledFunc))
	for addr, fn := range k.precompiledFunc {
		precompile := fn(ctx)
		if !bytes.Equal(addr.Bytes(), precompile.Address().Bytes()) {
			panic("precompile address mismatch")
		}
		addrs = append(addrs, addr)
		precompiles[addr] = precompile
	}
	if k.erc20Keeper != nil {
		params := k.erc20Keeper.GetParams(ctx)
		erc20Addrs := append([]string{}, params.DynamicPrecompiles...)
		erc20Addrs = append(erc20Addrs, params.NativePrecompiles...)

		for _, addrHex := range erc20Addrs {
			addr := common.HexToAddress(addrHex)
			if _, ok := precompiles[addr]; ok {
				continue
			}

			tokenPairID := k.erc20Keeper.GetERC20Map(ctx, addr)
			tokenPair, found := k.erc20Keeper.GetTokenPair(ctx, tokenPairID)
			if !found || !tokenPair.Enabled {
				continue
			}

			precompiles[addr] = precompileserc20.NewPrecompiledContract(ctx, tokenPair, k.bankKeeper, k.erc20Keeper)
			addrs = append(addrs, addr)
		}
	}
	sort.SliceStable(addrs, func(i, j int) bool {
		return bytes.Compare(addrs[i].Bytes(), addrs[j].Bytes()) < 0
	})
	return addrs, precompiles
}

type PrecompiledContractFunc func(ctx sdk.Context) vm.PrecompiledContract

// WithPrecompiled sets the available precompiled contracts.
func (k *Keeper) WithPrecompiled(precompiledFunc map[common.Address]PrecompiledContractFunc) *Keeper {
	if k.precompiledFunc != nil {
		panic("available precompiles map already set")
	}
	if len(precompiledFunc) == 0 {
		panic("empty precompiles contract map")
	}
	k.precompiledFunc = maps.Clone(precompiledFunc)
	return k
}

func BerlinPrecompiled() map[common.Address]PrecompiledContractFunc {
	precompiledFunc := make(map[common.Address]PrecompiledContractFunc, len(vm.PrecompiledContractsBerlin))
	for addr, precompiled := range vm.PrecompiledContractsBerlin {
		// wrap the precompiled contract to a function
		fn := func(precompiled vm.PrecompiledContract) PrecompiledContractFunc {
			return func(_ sdk.Context) vm.PrecompiledContract {
				return precompiled
			}
		}
		precompiledFunc[addr] = fn(precompiled)
	}
	return precompiledFunc
}
