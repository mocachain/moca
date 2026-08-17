package types

import (
	"errors"
	"fmt"
	"math/big"
	"strings"

	stdmath "math"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"gopkg.in/yaml.v2"
)

// SP params default values
const (
	// Default deposit denom
	DefaultDepositDenom = "amoca"
	// DefaultNumOfHistoricalBlocksForMaintenanceRecords The oldest block from current will be kept of for SP maintenance records
	DefaultNumOfHistoricalBlocksForMaintenanceRecords = 864000
	// DefaultMaintenanceDurationQuota is the total allowed time for a SP to be in Maintenance mode within DefaultNumOfHistoricalBlocksForMaintenanceRecords
	DefaultMaintenanceDurationQuota = 21600 // 6 hour
	// DefaultNumOfLockUpBlocksForMaintenance defines blocks difference which Sp update itself to Maintenance mode is allowed
	DefaultNumOfLockUpBlocksForMaintenance = 21600
	// DefaultUpdateGlobalPriceInterval defines the default time duration for updating global storage price
	DefaultUpdateGlobalPriceInterval uint64 = 0 // 0 means the global price will be updated at the first day of each month
	// UpdatePriceDisallowedDays defines the days, counting backward from the end of a month, in which sp is not allowed to update its price
	DefaultUpdatePriceDisallowedDays uint32 = 2
)

var (
	// DefaultMinDeposit defines the minimum deposit amount for all storage provider
	DefaultMinDeposit = math.NewIntFromBigInt(new(big.Int).Mul(big.NewInt(10000), big.NewInt(1e18)))
	// DefaultSecondarySpStorePriceRatio is 12%
	DefaultSecondarySpStorePriceRatio = math.LegacyNewDecFromIntWithPrec(math.NewInt(12), 2)
)

var (
	KeyDepositDenom                               = []byte("DepositDenom")
	KeyMinDeposit                                 = []byte("MinDeposit")
	KeySecondarySpStorePriceRatio                 = []byte("SecondarySpStorePriceRatio")
	KeyNumOfHistoricalBlocksForMaintenanceRecords = []byte("NumOfHistoricalBlocksForMaintenanceRecords")
	KeyMaintenanceDurationQuota                   = []byte("MaintenanceDurationQuota")
	KeyNumOfLockUpBlocksForMaintenance            = []byte("NumOfLockUpBlocksForMaintenance")
	KeyUpdateGlobalPriceInterval                  = []byte("UpdateGlobalPriceInterval")
	KeyUpdatePriceDisallowedDays                  = []byte("UpdatePriceDisallowedDays")
)

// NewParams creates a new Params instance
func NewParams(depositDenom string, minDeposit math.Int, secondarySpStorePriceRatio math.LegacyDec,
	historicalBlocksForMaintenanceRecords, maintenanceDurationQuota, lockUpBlocksForMaintenance int64,
	updateGlobalPriceInterval uint64, updatePriceDisallowedDays uint32,
) Params {
	return Params{
		DepositDenom:               depositDenom,
		MinDeposit:                 minDeposit,
		SecondarySpStorePriceRatio: secondarySpStorePriceRatio,
		NumOfHistoricalBlocksForMaintenanceRecords: historicalBlocksForMaintenanceRecords,
		MaintenanceDurationQuota:                   maintenanceDurationQuota,
		NumOfLockupBlocksForMaintenance:            lockUpBlocksForMaintenance,
		UpdateGlobalPriceInterval:                  updateGlobalPriceInterval,
		UpdatePriceDisallowedDays:                  updatePriceDisallowedDays,
	}
}

// DefaultParams returns a default set of parameters
func DefaultParams() Params {
	return NewParams(DefaultDepositDenom, DefaultMinDeposit, DefaultSecondarySpStorePriceRatio,
		DefaultNumOfHistoricalBlocksForMaintenanceRecords, DefaultMaintenanceDurationQuota, DefaultNumOfLockUpBlocksForMaintenance,
		DefaultUpdateGlobalPriceInterval, DefaultUpdatePriceDisallowedDays)
}

// Validate validates the set of params
func (p Params) Validate() error {
	if err := validateDepositDenom(p.DepositDenom); err != nil {
		return err
	}

	if err := validateMinDeposit(p.MinDeposit); err != nil {
		return err
	}

	if err := validateSecondarySpStorePriceRatio(p.SecondarySpStorePriceRatio); err != nil {
		return err
	}
	if err := validateHistoricalBlocksForMaintenanceRecords(p.NumOfHistoricalBlocksForMaintenanceRecords); err != nil {
		return err
	}
	if err := validateMaintenanceDurationQuota(p.MaintenanceDurationQuota); err != nil {
		return err
	}
	if err := validateLockUpBlocksForMaintenance(p.NumOfLockupBlocksForMaintenance); err != nil {
		return err
	}
	if err := validateUpdateGlobalPriceInterval(p.UpdateGlobalPriceInterval); err != nil {
		return err
	}
	if err := validateUpdatePriceDisallowedDays(p.UpdatePriceDisallowedDays); err != nil {
		return err
	}

	return nil
}

// String implements the Stringer interface.
func (p Params) String() string {
	out, _ := yaml.Marshal(p)
	return string(out)
}

func validateDepositDenom(i interface{}) error {
	v, ok := i.(string)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if strings.TrimSpace(v) == "" {
		return errors.New("deposit denom cannot be blank")
	}

	if err := sdk.ValidateDenom(v); err != nil {
		return err
	}

	return nil
}

func validateMinDeposit(i interface{}) error {
	v, ok := i.(math.Int)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}

	if v.IsNil() {
		return fmt.Errorf("minimum deposit amount cannot be nil")
	}

	if v.IsNegative() {
		return fmt.Errorf("minimum deposit amount cannot be lower than 0")
	}

	return nil
}

func validateSecondarySpStorePriceRatio(i interface{}) error {
	v, ok := i.(math.LegacyDec)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v.IsNil() || !v.IsPositive() || v.GT(math.LegacyOneDec()) {
		return fmt.Errorf("invalid secondary sp store price ratio")
	}
	return nil
}

func validateHistoricalBlocksForMaintenanceRecords(i interface{}) error {
	v, ok := i.(int64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v == 0 {
		return errors.New("HistoricalBlocksForMaintenanceRecords cannot be zero")
	}
	return nil
}

func validateMaintenanceDurationQuota(i interface{}) error {
	v, ok := i.(int64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v == 0 {
		return errors.New("MaintenanceDurationQuota cannot be zero")
	}
	return nil
}

func validateLockUpBlocksForMaintenance(i interface{}) error {
	v, ok := i.(int64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	if v == 0 {
		return errors.New("LockUpBlocksForMaintenance cannot be zero")
	}
	return nil
}

func validateUpdateGlobalPriceInterval(i interface{}) error {
	v, ok := i.(uint64)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	// The interval is compared against a duration in seconds as an int64
	// (x/sp/abci.go), so a value past that range wraps negative and makes the
	// comparison trivially true, updating the global price every block.
	if v > stdmath.MaxInt64 {
		return fmt.Errorf("update global price interval too large: %d", v)
	}
	return nil
}

func validateUpdatePriceDisallowedDays(i interface{}) error {
	_, ok := i.(uint32)
	if !ok {
		return fmt.Errorf("invalid parameter type: %T", i)
	}
	return nil
}
