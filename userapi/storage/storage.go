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

package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/neilalexander/harmony/internal/sqlutil"

	"github.com/neilalexander/harmony/setup/config"
	"github.com/neilalexander/harmony/userapi/storage/postgres"
)

// NewUserDatabase opens a new Postgres or Sqlite database (based on dataSourceName scheme)
// and sets postgres connection parameters
func NewUserDatabase(
	ctx context.Context,
	conMan *sqlutil.Connections,
	dbProperties *config.DatabaseOptions,
	serverName spec.ServerName,
	bcryptCost int,
	loginTokenLifetime time.Duration,
	serverNoticesLocalpart string,
) (UserDatabase, error) {
	switch {
	case dbProperties.ConnectionString.IsPostgres():
		return postgres.NewDatabase(ctx, conMan, dbProperties, serverName, bcryptCost, loginTokenLifetime, serverNoticesLocalpart)
	default:
		return nil, fmt.Errorf("unexpected database type")
	}
}

// NewKeyDatabase opens a new Postgres or Sqlite database (base on dataSourceName) scheme)
// and sets postgres connection parameters.
func NewKeyDatabase(conMan *sqlutil.Connections, dbProperties *config.DatabaseOptions) (KeyDatabase, error) {
	switch {
	case dbProperties.ConnectionString.IsPostgres():
		return postgres.NewKeyDatabase(conMan, dbProperties)
	default:
		return nil, fmt.Errorf("unexpected database type")
	}
}
