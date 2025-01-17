package integration

import (
	"testing"

	"github.com/shantanu-hashcash/go/clients/auroraclient"
	"github.com/shantanu-hashcash/go/services/aurora/internal/test/integration"
	"github.com/shantanu-hashcash/go/txnbuild"
	"github.com/shantanu-hashcash/go/xdr"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestP19MetaTransaction(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{
		ProtocolVersion:  19,
		EnableSorobanRPC: false,
	})

	masterAccount, err := itest.Client().AccountDetail(auroraclient.AccountRequest{
		AccountID: itest.Master().Address(),
	})
	require.NoError(t, err)

	op := &txnbuild.Payment{
		SourceAccount: itest.Master().Address(),
		Destination:   itest.Master().Address(),
		Asset:         txnbuild.NativeAsset{},
		Amount:        "10",
	}

	clientTx := itest.MustSubmitOperations(&masterAccount, itest.Master(), op)

	var txMetaResult xdr.TransactionMeta
	err = xdr.SafeUnmarshalBase64(clientTx.ResultMetaXdr, &txMetaResult)
	require.NoError(t, err)

	assert.Greater(t, len(txMetaResult.MustV2().Operations), 0)
	assert.Greater(t, len(txMetaResult.MustV2().TxChangesBefore), 0)
	// TODO figure out how to generate TxChangesAfter also
	//assert.Greater(t, len(txMetaResult.MustV2().TxChangesAfter), 0)
}

func TestP19MetaDisabledTransaction(t *testing.T) {
	itest := integration.NewTest(t, integration.Config{
		ProtocolVersion:    19,
		AuroraEnvironment: map[string]string{"SKIP_TXMETA": "TRUE"},
		EnableSorobanRPC:   false,
	})

	masterAccount, err := itest.Client().AccountDetail(auroraclient.AccountRequest{
		AccountID: itest.Master().Address(),
	})
	require.NoError(t, err)

	op := &txnbuild.Payment{
		SourceAccount: itest.Master().Address(),
		Destination:   itest.Master().Address(),
		Asset:         txnbuild.NativeAsset{},
		Amount:        "10",
	}

	clientTx := itest.MustSubmitOperations(&masterAccount, itest.Master(), op)

	assert.Empty(t, clientTx.ResultMetaXdr)
}

func TestP20MetaTransaction(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		ProtocolVersion:  20,
		EnableSorobanRPC: true,
	})

	// establish which account will be contract owner, and load it's current seq
	sourceAccount, err := itest.Client().AccountDetail(auroraclient.AccountRequest{
		AccountID: itest.Master().Address(),
	})
	require.NoError(t, err)

	installContractOp := assembleInstallContractCodeOp(t, itest.Master().Address(), add_u64_contract)
	preFlightOp, minFee := itest.PreflightHostFunctions(&sourceAccount, *installContractOp)
	clientTx := itest.MustSubmitOperationsWithFee(&sourceAccount, itest.Master(), minFee+txnbuild.MinBaseFee, &preFlightOp)

	var txMetaResult xdr.TransactionMeta
	err = xdr.SafeUnmarshalBase64(clientTx.ResultMetaXdr, &txMetaResult)
	require.NoError(t, err)

	assert.Greater(t, len(txMetaResult.MustV3().Operations), 0)
	assert.NotNil(t, txMetaResult.MustV3().SorobanMeta)
	assert.Greater(t, len(txMetaResult.MustV3().TxChangesAfter), 0)
	assert.Greater(t, len(txMetaResult.MustV3().TxChangesBefore), 0)
}

func TestP20MetaDisabledTransaction(t *testing.T) {
	if integration.GetCoreMaxSupportedProtocol() < 20 {
		t.Skip("This test run does not support less than Protocol 20")
	}

	itest := integration.NewTest(t, integration.Config{
		ProtocolVersion:    20,
		AuroraEnvironment: map[string]string{"SKIP_TXMETA": "TRUE"},
		EnableSorobanRPC:   true,
	})

	// establish which account will be contract owner, and load it's current seq
	sourceAccount, err := itest.Client().AccountDetail(auroraclient.AccountRequest{
		AccountID: itest.Master().Address(),
	})
	require.NoError(t, err)

	installContractOp := assembleInstallContractCodeOp(t, itest.Master().Address(), add_u64_contract)
	preFlightOp, minFee := itest.PreflightHostFunctions(&sourceAccount, *installContractOp)
	clientTx := itest.MustSubmitOperationsWithFee(&sourceAccount, itest.Master(), minFee+txnbuild.MinBaseFee, &preFlightOp)

	assert.Empty(t, clientTx.ResultMetaXdr)
}
