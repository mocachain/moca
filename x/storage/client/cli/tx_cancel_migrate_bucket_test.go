package cli_test

import (
	"github.com/mocachain/moca/v2/x/storage/client/cli"
)

// TestCmdCancelMigrateBucket exercises CmdCancelMigrateBucket up to the private-key gate:
// local arg parsing and client-context setup all succeed, and the command only fails once it
// tries to build a key manager from an empty private key (before any network dial happens).
func (s *CLITestSuite) TestCmdCancelMigrateBucket() {
	cmd := cli.GetTxCmd()

	args := []string{
		"cancel-migrate-bucket",
		"test-bucket",
		"--privatekey", "",
	}

	s.execExpectError(cmd, args)
}
