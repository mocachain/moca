package types

import "fmt"

func NewGenesisState(params Params, pairs []TokenPair, allowances []Allowance) GenesisState {
	return GenesisState{
		Params:     params,
		TokenPairs: pairs,
		Allowances: allowances,
	}
}

func DefaultGenesisState() *GenesisState {
	return &GenesisState{
		Params:     DefaultParams(),
		TokenPairs: []TokenPair{},
		Allowances: []Allowance{},
	}
}

func (gs GenesisState) Validate() error {
	seenErc20 := make(map[string]bool)
	seenDenom := make(map[string]bool)

	for _, pair := range gs.TokenPairs {
		if seenErc20[pair.Erc20Address] {
			return fmt.Errorf("token ERC20 contract duplicated on genesis '%s'", pair.Erc20Address)
		}
		if seenDenom[pair.Denom] {
			return fmt.Errorf("coin denomination duplicated on genesis: '%s'", pair.Denom)
		}
		if err := pair.Validate(); err != nil {
			return err
		}
		seenErc20[pair.Erc20Address] = true
		seenDenom[pair.Denom] = true
	}

	if err := gs.Params.Validate(); err != nil {
		return fmt.Errorf("invalid params on genesis: %w", err)
	}
	if err := validatePrecompiles(gs.TokenPairs, gs.Params.DynamicPrecompiles); err != nil {
		return fmt.Errorf("invalid dynamic precompiles on genesis: %w", err)
	}
	if err := validatePrecompiles(gs.TokenPairs, gs.Params.NativePrecompiles); err != nil {
		return fmt.Errorf("invalid native precompiles on genesis: %w", err)
	}

	seenAllowance := make(map[string]bool)
	for _, allowance := range gs.Allowances {
		key := allowance.Erc20Address + allowance.Owner + allowance.Spender
		if seenAllowance[key] {
			return fmt.Errorf("duplicated allowance on genesis: %s", key)
		}
		if !seenErc20[allowance.Erc20Address] {
			return fmt.Errorf("allowance has no corresponding token pair on genesis: %s", allowance.Erc20Address)
		}
		if err := allowance.Validate(); err != nil {
			return fmt.Errorf("invalid allowance on genesis: %w", err)
		}
		seenAllowance[key] = true
	}

	return nil
}

func validatePrecompiles(tokenPairs []TokenPair, precompiles []string) error {
	for _, precompile := range precompiles {
		if !hasActiveTokenPair(tokenPairs, precompile) {
			return fmt.Errorf("precompile address '%s' not found in token pairs", precompile)
		}
	}
	return nil
}

func hasActiveTokenPair(pairs []TokenPair, address string) bool {
	for _, pair := range pairs {
		if pair.Erc20Address == address && pair.Enabled {
			return true
		}
	}
	return false
}
