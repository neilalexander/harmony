// Copyright 2022 The Matrix.org Foundation C.I.C.
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

package internal

import (
	"context"
	"crypto/ed25519"
	"testing"

	"github.com/matrix-org/gomatrixserverlib/fclient"
	"github.com/matrix-org/gomatrixserverlib/spec"
	"github.com/neilalexander/harmony/federationapi/api"
	"github.com/neilalexander/harmony/federationapi/queue"
	"github.com/neilalexander/harmony/federationapi/statistics"
	"github.com/neilalexander/harmony/setup/config"
	"github.com/neilalexander/harmony/setup/process"
	"github.com/neilalexander/harmony/test"
	"github.com/stretchr/testify/assert"
)

type testFedClient struct {
	fclient.FederationClient
	queryKeysCalled bool
	claimKeysCalled bool
	shouldFail      bool
}

func (t *testFedClient) LookupRoomAlias(ctx context.Context, origin, s spec.ServerName, roomAlias string) (res fclient.RespDirectory, err error) {
	return fclient.RespDirectory{}, nil
}

func TestPerformWakeupServers(t *testing.T) {
	testDB := test.NewInMemoryFederationDatabase()

	server := spec.ServerName("wakeup")
	testDB.AddServerToBlacklist(server)
	testDB.SetServerAssumedOffline(context.Background(), server)
	blacklisted, err := testDB.IsServerBlacklisted(server)
	assert.NoError(t, err)
	assert.True(t, blacklisted)
	offline, err := testDB.IsServerAssumedOffline(context.Background(), server)
	assert.NoError(t, err)
	assert.True(t, offline)

	_, key, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)
	cfg := config.FederationAPI{
		Matrix: &config.Global{
			SigningIdentity: fclient.SigningIdentity{
				ServerName: "relay",
				KeyID:      "ed25519:1",
				PrivateKey: key,
			},
		},
	}
	fedClient := &testFedClient{}
	stats := statistics.NewStatistics(testDB, FailuresUntilBlacklist)
	queues := queue.NewOutgoingQueues(
		testDB, process.NewProcessContext(),
		false,
		cfg.Matrix.ServerName, fedClient, &stats,
		nil,
	)
	fedAPI := NewFederationInternalAPI(
		testDB, &cfg, nil, fedClient, &stats, nil, queues, nil,
	)

	req := api.PerformWakeupServersRequest{
		ServerNames: []spec.ServerName{server},
	}
	res := api.PerformWakeupServersResponse{}
	err = fedAPI.PerformWakeupServers(context.Background(), &req, &res)
	assert.NoError(t, err)

	blacklisted, err = testDB.IsServerBlacklisted(server)
	assert.NoError(t, err)
	assert.False(t, blacklisted)
}

func TestPerformDirectoryLookup(t *testing.T) {
	testDB := test.NewInMemoryFederationDatabase()

	_, key, err := ed25519.GenerateKey(nil)
	assert.NoError(t, err)
	cfg := config.FederationAPI{
		Matrix: &config.Global{
			SigningIdentity: fclient.SigningIdentity{
				ServerName: "relay",
				KeyID:      "ed25519:1",
				PrivateKey: key,
			},
		},
	}
	fedClient := &testFedClient{}
	stats := statistics.NewStatistics(testDB, FailuresUntilBlacklist)
	queues := queue.NewOutgoingQueues(
		testDB, process.NewProcessContext(),
		false,
		cfg.Matrix.ServerName, fedClient, &stats,
		nil,
	)
	fedAPI := NewFederationInternalAPI(
		testDB, &cfg, nil, fedClient, &stats, nil, queues, nil,
	)

	req := api.PerformDirectoryLookupRequest{
		RoomAlias:  "room",
		ServerName: "server",
	}
	res := api.PerformDirectoryLookupResponse{}
	err = fedAPI.PerformDirectoryLookup(context.Background(), &req, &res)
	assert.NoError(t, err)
}
