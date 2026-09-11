package cli_test

import (
	"github.com/mocachain/moca/v2/testutil/sample"
	"github.com/mocachain/moca/v2/x/storage/client/cli"
)

// TestCmdSetBucketFlowRateLimit exercises CmdSetBucketFlowRateLimit's local validation of the
// payment-account / bucket-owner addresses and the flow-rate-limit integer, then the
// private-key gate all the CLI tx commands share. None of these cases ever reach the network.
func (s *CLITestSuite) TestCmdSetBucketFlowRateLimit() {
	validPaymentAcc := sample.RandAccAddressHex()
	validBucketOwner := sample.RandAccAddressHex()

	testCases := []struct {
		name          string
		bucketName    string
		paymentAcc    string
		bucketOwner   string
		flowRateLimit string
		errContains   string
	}{
		{
			name:          "reaches private key gate",
			bucketName:    "test-bucket",
			paymentAcc:    validPaymentAcc,
			bucketOwner:   validBucketOwner,
			flowRateLimit: "1000",
		},
		{
			name:          "invalid payment account",
			bucketName:    "test-bucket",
			paymentAcc:    "not-a-valid-address",
			bucketOwner:   validBucketOwner,
			flowRateLimit: "1000",
		},
		{
			name:          "invalid bucket owner",
			bucketName:    "test-bucket",
			paymentAcc:    validPaymentAcc,
			bucketOwner:   "not-a-valid-address",
			flowRateLimit: "1000",
		},
		{
			name:          "invalid flow rate limit",
			bucketName:    "test-bucket",
			paymentAcc:    validPaymentAcc,
			bucketOwner:   validBucketOwner,
			flowRateLimit: "not-a-number",
			errContains:   "invalid flow-rate-limit",
		},
	}

	for _, tc := range testCases {
		tc := tc

		s.Run(tc.name, func() {
			cmd := cli.GetTxCmd()

			args := []string{
				"set-bucket-flow-rate-limit",
				tc.bucketName,
				tc.paymentAcc,
				tc.bucketOwner,
				tc.flowRateLimit,
				"--privatekey", "",
			}

			err := s.execExpectError(cmd, args)
			if tc.errContains != "" {
				s.Require().Contains(err.Error(), tc.errContains)
			}
		})
	}
}
