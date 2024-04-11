package filters

import (
	"testing"

	"github.com/shantanu-hashcash/go/services/aurora/internal/db2/history"
	"github.com/shantanu-hashcash/go/services/aurora/internal/test"
)

func TestItGetsFilters(t *testing.T) {
	tt := test.Start(t)
	defer tt.Finish()
	test.ResetAuroraDB(t, tt.AuroraDB)
	q := &history.Q{tt.AuroraSession()}

	filtersService := NewFilters()

	ingestFilters := filtersService.GetFilters(q, tt.Ctx)

	// should be total of filters implemented in the system
	tt.Assert.Len(ingestFilters, 2)
}
