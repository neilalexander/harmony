// Copyright 2020 The Matrix.org Foundation C.I.C.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/lib/pq"
	"github.com/matrix-org/dendrite/internal"
	"github.com/matrix-org/dendrite/internal/sqlutil"
	"github.com/matrix-org/dendrite/userapi/api"
	"github.com/matrix-org/dendrite/userapi/storage/tables"
)

var fallbackKeysSchema = `
-- Stores one-time public keys for users
CREATE TABLE IF NOT EXISTS keyserver_fallback_keys (
    user_id TEXT NOT NULL,
	device_id TEXT NOT NULL,
	key_id TEXT NOT NULL,
	algorithm TEXT NOT NULL,
	ts_added_secs BIGINT NOT NULL,
	key_json TEXT NOT NULL,
	used BOOLEAN NOT NULL,
	-- Clobber based on tuple of user/device/algorithm.
    CONSTRAINT keyserver_fallback_keys_unique UNIQUE (user_id, device_id, algorithm)
);

CREATE INDEX IF NOT EXISTS keyserver_fallback_keys_idx ON keyserver_fallback_keys (user_id, device_id);
`

const upsertFallbackKeysSQL = "" +
	"INSERT INTO keyserver_fallback_keys (user_id, device_id, key_id, algorithm, ts_added_secs, key_json, used)" +
	" VALUES ($1, $2, $3, $4, $5, $6, false)" +
	" ON CONFLICT ON CONSTRAINT keyserver_fallback_keys_unique" +
	" DO UPDATE SET key_id = $3, key_json = $6, used = false"

const selectFallbackKeysSQL = "" +
	"SELECT concat(algorithm, ':', key_id) as algorithmwithid, key_json FROM keyserver_fallback_keys WHERE user_id=$1 AND device_id=$2 AND concat(algorithm, ':', key_id) = ANY($3);"

const selectFallbackUnusedAlgorithmsSQL = "" +
	"SELECT algorithm FROM keyserver_fallback_keys WHERE user_id = $1 AND device_id = $2 AND used = false"

const selectFallbackKeysByAlgorithmSQL = "" +
	"SELECT key_id, key_json FROM keyserver_fallback_keys WHERE user_id = $1 AND device_id = $2 AND algorithm = $3 ORDER BY used ASC LIMIT 1"

const deleteFallbackKeysSQL = "" +
	"DELETE FROM keyserver_fallback_keys WHERE user_id = $1 AND device_id = $2"

const updateFallbackKeyUsedSQL = "" +
	"UPDATE keyserver_fallback_keys SET used=true WHERE user_id = $1 AND device_id = $2 AND key_id = $3 AND algorithm = $4"

type fallbackKeysStatements struct {
	db                         *sql.DB
	upsertKeysStmt             *sql.Stmt
	selectKeysStmt             *sql.Stmt
	selectUnusedAlgorithmsStmt *sql.Stmt
	selectKeyByAlgorithmStmt   *sql.Stmt
	deleteFallbackKeysStmt     *sql.Stmt
	updateFallbackKeyUsedStmt  *sql.Stmt
}

func NewPostgresFallbackKeysTable(db *sql.DB) (tables.FallbackKeys, error) {
	s := &fallbackKeysStatements{
		db: db,
	}
	_, err := db.Exec(fallbackKeysSchema)
	if err != nil {
		return nil, err
	}
	return s, sqlutil.StatementList{
		{&s.upsertKeysStmt, upsertFallbackKeysSQL},
		{&s.selectKeysStmt, selectFallbackKeysSQL},
		{&s.selectUnusedAlgorithmsStmt, selectFallbackUnusedAlgorithmsSQL},
		{&s.selectKeyByAlgorithmStmt, selectFallbackKeysByAlgorithmSQL},
		{&s.deleteFallbackKeysStmt, deleteFallbackKeysSQL},
		{&s.updateFallbackKeyUsedStmt, updateFallbackKeyUsedSQL},
	}.Prepare(db)
}

func (s *fallbackKeysStatements) SelectFallbackKeys(ctx context.Context, userID, deviceID string, keyIDsWithAlgorithms []string) (map[string]json.RawMessage, error) {
	rows, err := s.selectKeysStmt.QueryContext(ctx, userID, deviceID, pq.Array(keyIDsWithAlgorithms))
	if err != nil {
		return nil, err
	}
	defer internal.CloseAndLogIfError(ctx, rows, "selectKeysStmt: rows.close() failed")

	result := make(map[string]json.RawMessage)
	var (
		algorithmWithID string
		keyJSONStr      string
	)
	for rows.Next() {
		if err := rows.Scan(&algorithmWithID, &keyJSONStr); err != nil {
			return nil, err
		}
		result[algorithmWithID] = json.RawMessage(keyJSONStr)
	}
	return result, rows.Err()
}

func (s *fallbackKeysStatements) SelectUnusedFallbackKeyAlgorithms(ctx context.Context, userID, deviceID string) ([]string, error) {
	rows, err := s.selectUnusedAlgorithmsStmt.QueryContext(ctx, userID, deviceID)
	if err != nil {
		return nil, err
	}
	defer internal.CloseAndLogIfError(ctx, rows, "selectKeysCountStmt: rows.close() failed")
	algos := []string{}
	for rows.Next() {
		var algorithm string
		if err = rows.Scan(&algorithm); err != nil {
			return nil, err
		}
		algos = append(algos, algorithm)
	}
	return algos, rows.Err()
}

func (s *fallbackKeysStatements) InsertFallbackKeys(ctx context.Context, txn *sql.Tx, keys api.FallbackKeys) ([]string, error) {
	now := time.Now().Unix()
	for keyIDWithAlgo, keyJSON := range keys.KeyJSON {
		algo, keyID := keys.Split(keyIDWithAlgo)
		_, err := sqlutil.TxStmt(txn, s.upsertKeysStmt).ExecContext(
			ctx, keys.UserID, keys.DeviceID, keyID, algo, now, string(keyJSON),
		)
		if err != nil {
			return nil, err
		}
	}
	return s.SelectUnusedFallbackKeyAlgorithms(ctx, keys.UserID, keys.DeviceID)
}

func (s *fallbackKeysStatements) DeleteFallbackKeys(ctx context.Context, txn *sql.Tx, userID, deviceID string) error {
	_, err := sqlutil.TxStmt(txn, s.deleteFallbackKeysStmt).ExecContext(ctx, userID, deviceID)
	return err
}

func (s *fallbackKeysStatements) SelectAndUpdateFallbackKey(
	ctx context.Context, txn *sql.Tx, userID, deviceID, algorithm string,
) (map[string]json.RawMessage, error) {
	var keyID string
	var keyJSON string
	err := sqlutil.TxStmtContext(ctx, txn, s.selectKeyByAlgorithmStmt).QueryRowContext(ctx, userID, deviceID, algorithm).Scan(&keyID, &keyJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	_, err = sqlutil.TxStmtContext(ctx, txn, s.updateFallbackKeyUsedStmt).ExecContext(ctx, userID, deviceID, algorithm, keyID)
	return map[string]json.RawMessage{
		algorithm + ":" + keyID: json.RawMessage(keyJSON),
	}, err
}
