// Copyright 2017 Vector Creations Ltd
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

package roomserver

import (
	"github.com/neilalexander/harmony/internal/caching"
	"github.com/neilalexander/harmony/internal/sqlutil"
	"github.com/neilalexander/harmony/setup/config"
	"github.com/neilalexander/harmony/setup/jetstream"
	"github.com/neilalexander/harmony/setup/process"
	"github.com/sirupsen/logrus"

	"github.com/neilalexander/harmony/roomserver/api"
	"github.com/neilalexander/harmony/roomserver/internal"
	"github.com/neilalexander/harmony/roomserver/storage"
)

// NewInternalAPI returns a concrete implementation of the internal API.
//
// Many of the methods provided by this API depend on access to a federation API, and so
// you may wish to call `SetFederationAPI` on the returned struct to avoid nil-dereference errors.
func NewInternalAPI(
	processContext *process.ProcessContext,
	cfg *config.Dendrite,
	cm *sqlutil.Connections,
	natsInstance *jetstream.NATSInstance,
	caches caching.RoomServerCaches,
	enableMetrics bool,
) api.RoomserverInternalAPI {
	roomserverDB, err := storage.Open(processContext.Context(), cm, &cfg.RoomServer.Database, caches)
	if err != nil {
		logrus.WithError(err).Panicf("failed to connect to room server db")
	}

	js, nc := natsInstance.Prepare(processContext, &cfg.Global.JetStream)

	return internal.NewRoomserverAPI(
		processContext, cfg, roomserverDB, js, nc, caches, enableMetrics,
	)
}
