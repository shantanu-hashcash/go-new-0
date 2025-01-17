package history

import (
	"context"
	"database/sql/driver"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/shantanu-hashcash/go/services/aurora/internal/test"
	"github.com/shantanu-hashcash/go/xdr"
)

func TestClaimableBalanceLoader(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetAuroraDB(t, tt.AuroraDB)
	session := tt.AuroraSession()

	var ids []string
	for i := 0; i < 100; i++ {
		balanceID := xdr.ClaimableBalanceId{
			Type: xdr.ClaimableBalanceIdTypeClaimableBalanceIdTypeV0,
			V0:   &xdr.Hash{byte(i)},
		}
		id, err := xdr.MarshalHex(balanceID)
		tt.Assert.NoError(err)
		ids = append(ids, id)
	}

	loader := NewClaimableBalanceLoader()
	var futures []FutureClaimableBalanceID
	for _, id := range ids {
		future := loader.GetFuture(id)
		futures = append(futures, future)
		_, err := future.Value()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), `invalid claimable balance loader state,`)
		duplicateFuture := loader.GetFuture(id)
		assert.Equal(t, future, duplicateFuture)
	}

	err := loader.Exec(context.Background(), session)
	assert.NoError(t, err)
	assert.Equal(t, LoaderStats{
		Total:    100,
		Inserted: 100,
	}, loader.Stats())
	assert.Panics(t, func() {
		loader.GetFuture("not-present")
	})

	q := &Q{session}
	for i, id := range ids {
		future := futures[i]
		var internalID driver.Value
		internalID, err = future.Value()
		assert.NoError(t, err)
		var cb HistoryClaimableBalance
		cb, err = q.ClaimableBalanceByID(context.Background(), id)
		assert.NoError(t, err)
		assert.Equal(t, cb.BalanceID, id)
		assert.Equal(t, cb.InternalID, internalID)
	}

	futureCb := &FutureClaimableBalanceID{id: "not-present", loader: loader}
	_, err = futureCb.Value()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), `was not found`)
}
