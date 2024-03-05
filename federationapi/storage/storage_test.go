package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/matrix-org/dendrite/federationapi/storage"
	"github.com/matrix-org/dendrite/internal/caching"
	"github.com/matrix-org/dendrite/internal/sqlutil"
	"github.com/matrix-org/dendrite/setup/config"
	"github.com/matrix-org/dendrite/test"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/stretchr/testify/assert"
)

func mustCreateFederationDatabase(t *testing.T, dbType test.DBType) (storage.Database, func()) {
	caches := caching.NewRistrettoCache(8*1024*1024, time.Hour, false)
	connStr, dbClose := test.PrepareDBConnectionString(t, dbType)
	ctx := context.Background()
	cm := sqlutil.NewConnectionManager(nil, config.DatabaseOptions{})
	db, err := storage.NewDatabase(ctx, cm, &config.DatabaseOptions{
		ConnectionString: config.DataSource(connStr),
	}, caches, func(server spec.ServerName) bool { return server == "localhost" })
	if err != nil {
		t.Fatalf("NewDatabase returned %s", err)
	}
	return db, func() {
		dbClose()
	}
}

func TestExpireEDUs(t *testing.T) {
	var expireEDUTypes = map[string]time.Duration{
		spec.MReceipt: 0,
	}

	ctx := context.Background()
	destinations := map[spec.ServerName]struct{}{"localhost": {}}
	test.WithAllDatabases(t, func(t *testing.T, dbType test.DBType) {
		db, close := mustCreateFederationDatabase(t, dbType)
		defer close()
		// insert some data
		for i := 0; i < 100; i++ {
			receipt, err := db.StoreJSON(ctx, "{}")
			assert.NoError(t, err)

			err = db.AssociateEDUWithDestinations(ctx, destinations, receipt, spec.MReceipt, expireEDUTypes)
			assert.NoError(t, err)
		}
		// add data without expiry
		receipt, err := db.StoreJSON(ctx, "{}")
		assert.NoError(t, err)

		// m.read_marker gets the default expiry of 24h, so won't be deleted further down in this test
		err = db.AssociateEDUWithDestinations(ctx, destinations, receipt, "m.read_marker", expireEDUTypes)
		assert.NoError(t, err)

		// Delete expired EDUs
		err = db.DeleteExpiredEDUs(ctx)
		assert.NoError(t, err)

		// verify the data is gone
		data, err := db.GetPendingEDUs(ctx, "localhost", 100)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(data))

		// check that m.direct_to_device is never expired
		receipt, err = db.StoreJSON(ctx, "{}")
		assert.NoError(t, err)

		err = db.AssociateEDUWithDestinations(ctx, destinations, receipt, spec.MDirectToDevice, expireEDUTypes)
		assert.NoError(t, err)

		err = db.DeleteExpiredEDUs(ctx)
		assert.NoError(t, err)

		// We should get two EDUs, the m.read_marker and the m.direct_to_device
		data, err = db.GetPendingEDUs(ctx, "localhost", 100)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(data))
	})
}
