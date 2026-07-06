package server

import (
	"fmt"
	"time"

	"cosmossdk.io/log"

	"github.com/cosmos/cosmos-sdk/client"
	sdkcrypto "github.com/cosmos/cosmos-sdk/crypto"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/eth/ethsecp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	mocatypes "github.com/mocachain/moca/v2/types"
)

// personalForkAPI overrides the two personal-namespace methods that create
// keyring records. cosmos/evm's implementations construct cosmos/evm's own
// ethsecp256k1 key types, which moca's codecs do not register (moca's SDK fork
// registers cosmos.crypto.eth.ethsecp256k1 / amino ethereum/PrivKeyEthSecp256k1
// instead): personal_importRawKey would crash on amino-marshal and
// personal_newAccount would write a keyring record the fork keyring cannot
// read back. These ports of the pre-migration implementations keep every
// keyring record fork-typed. All other personal_* methods remain cosmos/evm's.
type personalForkAPI struct {
	clientCtx  client.Context
	logger     log.Logger
	hdPathIter mocatypes.HDPathIterator
}

func newPersonalForkAPI(logger log.Logger, clientCtx client.Context) *personalForkAPI {
	basePath := sdk.GetConfig().GetFullBIP44Path()

	iterator, err := mocatypes.NewHDPathIterator(basePath, true)
	if err != nil {
		panic(err)
	}

	return &personalForkAPI{
		clientCtx:  clientCtx,
		logger:     logger.With("api", "personal"),
		hdPathIter: iterator,
	}
}

// ImportRawKey armors and encrypts a given raw hex encoded ECDSA key and
// stores it into the key directory. The name of the key will have the format
// "personal_<length-keys>", where <length-keys> is the total number of keys
// stored on the keyring.
//
// NOTE: The key will be both armored and encrypted using the same passphrase.
func (p *personalForkAPI) ImportRawKey(privkey, password string) (common.Address, error) {
	p.logger.Debug("personal_importRawKey")

	priv, err := crypto.HexToECDSA(privkey)
	if err != nil {
		return common.Address{}, err
	}

	privKey := &ethsecp256k1.PrivKey{Key: crypto.FromECDSA(priv)}

	addr := sdk.AccAddress(privKey.PubKey().Address().Bytes())
	ethereumAddr := common.BytesToAddress(addr)

	// return if the key has already been imported
	if _, err := p.clientCtx.Keyring.KeyByAddress(addr); err == nil {
		return ethereumAddr, nil
	}

	// ignore error as we only care about the length of the list
	list, _ := p.clientCtx.Keyring.List() // #nosec G703
	privKeyName := fmt.Sprintf("personal_%d", len(list))

	armor := sdkcrypto.EncryptArmorPrivKey(privKey, password, ethsecp256k1.KeyType)

	if err := p.clientCtx.Keyring.ImportPrivKey(privKeyName, armor, password); err != nil {
		return common.Address{}, err
	}

	p.logger.Info("key successfully imported", "name", privKeyName, "address", ethereumAddr.String())

	return ethereumAddr, nil
}

// NewAccount will create a new account and return the address for the new
// account.
func (p *personalForkAPI) NewAccount(password string) (common.Address, error) {
	p.logger.Debug("personal_newAccount")

	name := "key_" + time.Now().UTC().Format(time.RFC3339)
	hdPath := p.hdPathIter()

	info, _, err := p.clientCtx.Keyring.NewMnemonic(name, keyring.English, hdPath.String(), password, hd.EthSecp256k1)
	if err != nil {
		return common.Address{}, err
	}

	pubKey, err := info.GetPubKey()
	if err != nil {
		return common.Address{}, err
	}
	addr := common.BytesToAddress(pubKey.Address().Bytes())
	p.logger.Info("Your new key was generated", "address", addr.String())
	p.logger.Info("Please remember your password!")
	return addr, nil
}
