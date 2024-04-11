package ingest

import (
	"bytes"
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/hcnet/go/ingest"
	"github.com/hcnet/go/ingest/ledgerbackend"
	"github.com/hcnet/go/services/aurora/internal/db2/history"
	"github.com/hcnet/go/support/db"
	"github.com/hcnet/go/support/errors"
	logpkg "github.com/hcnet/go/support/log"
	strtime "github.com/hcnet/go/support/time"
	"github.com/hcnet/go/xdr"
)

var (
	issuer   = xdr.MustAddress("GBRPYHIL2CI3FNQ4BXLFMNDLFJUNPU2HY3ZMFSHONUCEOASW7QC7OX2H")
	usdAsset = xdr.Asset{
		Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
		AlphaNum4: &xdr.AlphaNum4{
			AssetCode: [4]byte{'u', 's', 'd', 0},
			Issuer:    issuer,
		},
	}

	nativeAsset = xdr.Asset{
		Type: xdr.AssetTypeAssetTypeNative,
	}

	eurAsset = xdr.Asset{
		Type: xdr.AssetTypeAssetTypeCreditAlphanum4,
		AlphaNum4: &xdr.AlphaNum4{
			AssetCode: [4]byte{'e', 'u', 'r', 0},
			Issuer:    issuer,
		},
	}
	eurOffer = xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(4),
		Buying:   eurAsset,
		Selling:  nativeAsset,
		Price: xdr.Price{
			N: 1,
			D: 1,
		},
		Flags:  1,
		Amount: xdr.Int64(500),
	}
	twoEurOffer = xdr.OfferEntry{
		SellerId: issuer,
		OfferId:  xdr.Int64(5),
		Buying:   eurAsset,
		Selling:  nativeAsset,
		Price: xdr.Price{
			N: 2,
			D: 1,
		},
		Flags:  2,
		Amount: xdr.Int64(500),
	}
)

func TestCheckVerifyStateVersion(t *testing.T) {
	assert.Equal(
		t,
		CurrentVersion,
		stateVerifierExpectedIngestionVersion,
		"State verifier is outdated, update it, then update stateVerifierExpectedIngestionVersion value",
	)
}

func TestLedgerEligibleForStateVerification(t *testing.T) {
	checker := ledgerEligibleForStateVerification(64, 3)
	for ledger := uint32(1); ledger < 64*6; ledger++ {
		run := checker(ledger)
		expected := (ledger+1)%(64*3) == 0
		assert.Equal(t, expected, run)
	}
}

func TestNewSystem(t *testing.T) {
	config := Config{
		HistorySession:           &db.Session{DB: &sqlx.DB{}},
		DisableStateVerification: true,
		HistoryArchiveURLs:       []string{"https://history.hcnet.org/prd/core-live/core_live_001"},
		CheckpointFrequency:      64,
	}

	sIface, err := NewSystem(config)
	assert.NoError(t, err)
	system := sIface.(*system)

	assert.Equal(t, config, system.config)
	assert.Equal(t, config.DisableStateVerification, system.disableStateVerification)

	assert.Equal(t, config, system.runner.(*ProcessorRunner).config)
	assert.Equal(t, system.ctx, system.runner.(*ProcessorRunner).ctx)
	assert.Equal(t, system.maxLedgerPerFlush, MaxLedgersPerFlush)
}

func TestStateMachineRunReturnsUnexpectedTransaction(t *testing.T) {
	historyQ := &mockDBQ{}
	system := &system{
		historyQ: historyQ,
		ctx:      context.Background(),
	}

	historyQ.On("GetTx").Return(&sqlx.Tx{}).Once()

	assert.PanicsWithValue(t, "unexpected transaction", func() {
		system.Run()
	})
}

func TestStateMachineTransition(t *testing.T) {
	historyQ := &mockDBQ{}
	system := &system{
		historyQ: historyQ,
		ctx:      context.Background(),
	}

	historyQ.On("GetTx").Return(nil).Once()
	historyQ.On("Begin", mock.Anything).Return(errors.New("my error")).Once()
	historyQ.On("GetTx").Return(&sqlx.Tx{}).Once()

	assert.PanicsWithValue(t, "unexpected transaction", func() {
		system.Run()
	})
}

func TestContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	historyQ := &mockDBQ{}
	system := &system{
		historyQ: historyQ,
		ctx:      ctx,
	}

	historyQ.On("GetTx").Return(nil).Once()
	historyQ.On("Begin", mock.AnythingOfType("*context.cancelCtx")).Return(errors.New("my error")).Once()

	cancel()
	assert.NoError(t, system.runStateMachine(startState{}))
}

// TestStateMachineRunReturnsErrorWhenNextStateIsShutdownWithError checks if the
// state that goes to shutdownState and returns an error will make `run` function
// return that error. This is essential because some commands rely on this to return
// non-zero exit code.
func TestStateMachineRunReturnsErrorWhenNextStateIsShutdownWithError(t *testing.T) {
	historyQ := &mockDBQ{}
	system := &system{
		ctx:      context.Background(),
		historyQ: historyQ,
	}

	historyQ.On("GetTx").Return(nil).Once()

	err := system.runStateMachine(verifyRangeState{})
	assert.Error(t, err)
	assert.EqualError(t, err, "invalid range: [0, 0]")
}

func TestMaybeVerifyStateGetExpStateInvalidError(t *testing.T) {
	historyQ := &mockDBQ{}
	system := &system{
		historyQ:                     historyQ,
		ctx:                          context.Background(),
		runStateVerificationOnLedger: ledgerEligibleForStateVerification(64, 1),
	}

	var out bytes.Buffer
	logger := logpkg.New()
	logger.SetOutput(&out)
	done := logger.StartTest(logpkg.InfoLevel)

	oldLogger := log
	log = logger
	defer func() { log = oldLogger }()

	historyQ.On("GetExpStateInvalid", system.ctx).Return(false, db.ErrCancelled).Once()
	system.maybeVerifyState(63)
	system.wg.Wait()

	historyQ.On("GetExpStateInvalid", system.ctx).Return(false, context.Canceled).Once()
	system.maybeVerifyState(63)
	system.wg.Wait()

	logged := done()
	assert.Len(t, logged, 0)

	// Ensure state verifier does not start also for any other error
	historyQ.On("GetExpStateInvalid", system.ctx).Return(false, errors.New("my error")).Once()
	system.maybeVerifyState(63)
	system.wg.Wait()

	historyQ.AssertExpectations(t)
}
func TestMaybeVerifyInternalDBErrCancelOrContextCanceled(t *testing.T) {
	historyQ := &mockDBQ{}
	system := &system{
		historyQ:                     historyQ,
		ctx:                          context.Background(),
		runStateVerificationOnLedger: ledgerEligibleForStateVerification(64, 1),
	}

	var out bytes.Buffer
	logger := logpkg.New()
	logger.SetOutput(&out)
	done := logger.StartTest(logpkg.InfoLevel)

	oldLogger := log
	log = logger
	defer func() { log = oldLogger }()

	historyQ.On("GetExpStateInvalid", system.ctx, mock.Anything).Return(false, nil).Twice()
	historyQ.On("Rollback").Return(nil).Twice()
	historyQ.On("CloneIngestionQ").Return(historyQ).Twice()

	historyQ.On("BeginTx", mock.Anything, mock.Anything).Return(db.ErrCancelled).Once()
	system.maybeVerifyState(63)
	system.wg.Wait()

	historyQ.On("BeginTx", mock.Anything, mock.Anything).Return(context.Canceled).Once()
	system.maybeVerifyState(63)
	system.wg.Wait()

	logged := done()

	// it logs "State verification finished" twice, but no errors
	assert.Len(t, logged, 0)

	historyQ.AssertExpectations(t)
}

func TestCurrentStateRaceCondition(t *testing.T) {
	historyQ := &mockDBQ{}
	s := &system{
		historyQ: historyQ,
		ctx:      context.Background(),
	}

	historyQ.On("GetTx").Return(nil)
	historyQ.On("Begin", s.ctx).Return(nil)
	historyQ.On("Rollback").Return(nil)
	historyQ.On("GetLastLedgerIngest", s.ctx).Return(uint32(1), nil)
	historyQ.On("GetIngestVersion", s.ctx).Return(CurrentVersion, nil)

	timer := time.NewTimer(2000 * time.Millisecond)
	getCh := make(chan bool, 1)
	doneCh := make(chan bool, 1)
	go func() {
		var state = buildState{checkpointLedger: 8,
			skipChecks: true,
			stop:       true}
		for range getCh {
			_ = s.runStateMachine(state)
		}
		close(doneCh)
	}()

loop:
	for {
		s.GetCurrentState()
		select {
		case <-timer.C:
			break loop
		default:
		}
		getCh <- true
	}
	close(getCh)
	<-doneCh
}

type mockDBQ struct {
	mock.Mock

	history.MockQAccounts
	history.MockQFilter
	history.MockQClaimableBalances
	history.MockQHistoryClaimableBalances
	history.MockQLiquidityPools
	history.MockQHistoryLiquidityPools
	history.MockQAssetStats
	history.MockQData
	history.MockQEffects
	history.MockQLedgers
	history.MockQOffers
	history.MockQOperations
	history.MockQSigners
	history.MockQTransactions
	history.MockQTrustLines
}

func (m *mockDBQ) Begin(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockDBQ) BeginTx(ctx context.Context, txOpts *sql.TxOptions) error {
	args := m.Called(ctx, txOpts)
	return args.Error(0)
}

func (m *mockDBQ) CloneIngestionQ() history.IngestionQ {
	args := m.Called()
	return args.Get(0).(history.IngestionQ)
}

func (m *mockDBQ) Commit() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockDBQ) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockDBQ) Rollback() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockDBQ) TryStateVerificationLock(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Get(0).(bool), args.Error(1)
}

func (m *mockDBQ) GetTx() *sqlx.Tx {
	args := m.Called()
	if args.Get(0) == nil {
		return nil
	}
	return args.Get(0).(*sqlx.Tx)
}

func (m *mockDBQ) GetLastLedgerIngest(ctx context.Context) (uint32, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint32), args.Error(1)
}

func (m *mockDBQ) GetOfferCompactionSequence(ctx context.Context) (uint32, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint32), args.Error(1)
}

func (m *mockDBQ) GetLiquidityPoolCompactionSequence(ctx context.Context) (uint32, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint32), args.Error(1)
}

func (m *mockDBQ) GetLastLedgerIngestNonBlocking(ctx context.Context) (uint32, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint32), args.Error(1)
}

func (m *mockDBQ) GetIngestVersion(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Get(0).(int), args.Error(1)
}

func (m *mockDBQ) UpdateLastLedgerIngest(ctx context.Context, sequence uint32) error {
	args := m.Called(ctx, sequence)
	return args.Error(0)
}

func (m *mockDBQ) UpdateExpStateInvalid(ctx context.Context, invalid bool) error {
	args := m.Called(ctx, invalid)
	return args.Error(0)
}

func (m *mockDBQ) UpdateIngestVersion(ctx context.Context, version int) error {
	args := m.Called(ctx, version)
	return args.Error(0)
}

func (m *mockDBQ) GetExpStateInvalid(ctx context.Context) (bool, error) {
	args := m.Called(ctx)
	return args.Get(0).(bool), args.Error(1)
}

func (m *mockDBQ) StreamAllOffers(ctx context.Context, callback func(history.Offer) error) error {
	a := m.Called(ctx, callback)
	return a.Error(0)
}

func (m *mockDBQ) GetLatestHistoryLedger(ctx context.Context) (uint32, error) {
	args := m.Called(ctx)
	return args.Get(0).(uint32), args.Error(1)
}

func (m *mockDBQ) TruncateIngestStateTables(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *mockDBQ) DeleteRangeAll(ctx context.Context, start, end int64) error {
	args := m.Called(ctx, start, end)
	return args.Error(0)
}

// Methods from interfaces duplicating methods:

func (m *mockDBQ) NewTransactionParticipantsBatchInsertBuilder() history.TransactionParticipantsBatchInsertBuilder {
	args := m.Called()
	return args.Get(0).(history.TransactionParticipantsBatchInsertBuilder)
}

func (m *mockDBQ) NewOperationParticipantBatchInsertBuilder() history.OperationParticipantBatchInsertBuilder {
	args := m.Called()
	return args.Get(0).(history.OperationParticipantBatchInsertBuilder)
}

func (m *mockDBQ) NewTradeBatchInsertBuilder() history.TradeBatchInsertBuilder {
	args := m.Called()
	return args.Get(0).(history.TradeBatchInsertBuilder)
}

func (m *mockDBQ) ReapLookupTables(ctx context.Context, offsets map[string]int64) (map[string]int64, map[string]int64, error) {
	args := m.Called(ctx, offsets)
	var r1, r2 map[string]int64
	if args.Get(0) != nil {
		r1 = args.Get(0).(map[string]int64)
	}
	if args.Get(1) != nil {
		r1 = args.Get(1).(map[string]int64)
	}
	return r1, r2, args.Error(2)
}

func (m *mockDBQ) RebuildTradeAggregationTimes(ctx context.Context, from, to strtime.Millis, roundingSlippageFilter int) error {
	args := m.Called(ctx, from, to, roundingSlippageFilter)
	return args.Error(0)
}

func (m *mockDBQ) RebuildTradeAggregationBuckets(ctx context.Context, fromLedger, toLedger uint32, roundingSlippageFilter int) error {
	args := m.Called(ctx, fromLedger, toLedger, roundingSlippageFilter)
	return args.Error(0)
}

func (m *mockDBQ) CreateAssets(ctx context.Context, assets []xdr.Asset, batchSize int) (map[string]history.Asset, error) {
	args := m.Called(ctx, assets)
	return args.Get(0).(map[string]history.Asset), args.Error(1)
}

func (m *mockDBQ) DeleteTransactionsFilteredTmpOlderThan(ctx context.Context, howOldInSeconds uint64) (int64, error) {
	args := m.Called(ctx, howOldInSeconds)
	return args.Get(0).(int64), args.Error(1)
}

type mockLedgerBackend struct {
	mock.Mock
}

func (m *mockLedgerBackend) GetLatestLedgerSequence(ctx context.Context) (sequence uint32, err error) {
	args := m.Called(ctx)
	return args.Get(0).(uint32), args.Error(1)
}

func (m *mockLedgerBackend) GetLedger(ctx context.Context, sequence uint32) (xdr.LedgerCloseMeta, error) {
	args := m.Called(ctx, sequence)
	return args.Get(0).(xdr.LedgerCloseMeta), args.Error(1)
}

func (m *mockLedgerBackend) PrepareRange(ctx context.Context, ledgerRange ledgerbackend.Range) error {
	args := m.Called(ctx, ledgerRange)
	return args.Error(0)
}

func (m *mockLedgerBackend) IsPrepared(ctx context.Context, ledgerRange ledgerbackend.Range) (bool, error) {
	args := m.Called(ctx, ledgerRange)
	return args.Get(0).(bool), args.Error(1)
}

func (m *mockLedgerBackend) Close() error {
	args := m.Called()
	return args.Error(0)
}

type mockProcessorsRunner struct {
	mock.Mock
}

func (m *mockProcessorsRunner) SetHistoryAdapter(historyAdapter historyArchiveAdapterInterface) {
	m.Called(historyAdapter)
}

func (m *mockProcessorsRunner) EnableMemoryStatsLogging() {
	m.Called()
}

func (m *mockProcessorsRunner) DisableMemoryStatsLogging() {
	m.Called()
}

func (m *mockProcessorsRunner) RunGenesisStateIngestion() (ingest.StatsChangeProcessorResults, error) {
	args := m.Called()
	return args.Get(0).(ingest.StatsChangeProcessorResults), args.Error(1)
}

func (m *mockProcessorsRunner) RunHistoryArchiveIngestion(
	checkpointLedger uint32,
	skipChecks bool,
	ledgerProtocolVersion uint32,
	bucketListHash xdr.Hash,
) (ingest.StatsChangeProcessorResults, error) {
	args := m.Called(checkpointLedger, skipChecks, ledgerProtocolVersion, bucketListHash)
	return args.Get(0).(ingest.StatsChangeProcessorResults), args.Error(1)
}

func (m *mockProcessorsRunner) RunAllProcessorsOnLedger(ledger xdr.LedgerCloseMeta) (
	ledgerStats,
	error,
) {
	args := m.Called(ledger)
	return args.Get(0).(ledgerStats),
		args.Error(1)
}

func (m *mockProcessorsRunner) RunTransactionProcessorsOnLedgers(ledgers []xdr.LedgerCloseMeta) error {
	args := m.Called(ledgers)
	return args.Error(0)
}

var _ ProcessorRunnerInterface = (*mockProcessorsRunner)(nil)

type mockHcnetCoreClient struct {
	mock.Mock
}

func (m *mockHcnetCoreClient) SetCursor(ctx context.Context, id string, cursor int32) error {
	args := m.Called(ctx, id, cursor)
	return args.Error(0)
}

var _ hcnetCoreClient = (*mockHcnetCoreClient)(nil)

type mockSystem struct {
	mock.Mock
}

func (m *mockSystem) Run() {
	m.Called()
}

func (m *mockSystem) Metrics() Metrics {
	args := m.Called()
	return args.Get(0).(Metrics)
}

func (m *mockSystem) RegisterMetrics(registry *prometheus.Registry) {
	m.Called(registry)
}

func (m *mockSystem) StressTest(numTransactions, changesPerTransaction int) error {
	args := m.Called(numTransactions, changesPerTransaction)
	return args.Error(0)
}

func (m *mockSystem) VerifyRange(fromLedger, toLedger uint32, verifyState bool) error {
	args := m.Called(fromLedger, toLedger, verifyState)
	return args.Error(0)
}

func (m *mockSystem) BuildState(sequence uint32, skipChecks bool) error {
	args := m.Called(sequence, skipChecks)
	return args.Error(0)
}

func (m *mockSystem) ReingestRange(ledgerRanges []history.LedgerRange, force bool, rebuildTradeAgg bool) error {
	args := m.Called(ledgerRanges, force, rebuildTradeAgg)
	return args.Error(0)
}

func (m *mockSystem) BuildGenesisState() error {
	args := m.Called()
	return args.Error(0)
}

func (m *mockSystem) GetCurrentState() State {
	args := m.Called()
	return args.Get(0).(State)
}

func (m *mockSystem) RebuildTradeAggregationBuckets(fromLedger, toLedger uint32) error {
	args := m.Called(fromLedger, toLedger)
	return args.Error(0)
}

func (m *mockSystem) Shutdown() {
	m.Called()
}

var _ System = (*mockSystem)(nil)
