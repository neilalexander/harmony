package util_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/matrix-org/util"
	"github.com/neilalexander/harmony/internal/gomatrixserverlib"
	"github.com/neilalexander/harmony/internal/gomatrixserverlib/spec"
	"github.com/neilalexander/harmony/internal/sqlutil"
	"github.com/neilalexander/harmony/syncapi/synctypes"
	"golang.org/x/crypto/bcrypt"

	"github.com/neilalexander/harmony/internal/pushgateway"
	"github.com/neilalexander/harmony/setup/config"
	"github.com/neilalexander/harmony/test"
	"github.com/neilalexander/harmony/userapi/api"
	"github.com/neilalexander/harmony/userapi/storage"
	userUtil "github.com/neilalexander/harmony/userapi/util"
)

func queryUserIDForSender(senderID spec.SenderID) (*spec.UserID, error) {
	if senderID == "" {
		return nil, nil
	}

	return spec.NewUserID(string(senderID), true)
}

func TestNotifyUserCountsAsync(t *testing.T) {
	alice := test.NewUser(t)
	aliceLocalpart, serverName, err := gomatrixserverlib.SplitID('@', alice.ID)
	if err != nil {
		t.Error(err)
	}
	ctx := context.Background()

	// Create a test room, just used to provide events
	room := test.NewRoom(t, alice)
	dummyEvent := room.Events()[len(room.Events())-1]

	appID := util.RandomString(8)
	pushKey := util.RandomString(8)

	test.WithAllDatabases(t, func(t *testing.T, dbType test.DBType) {
		receivedRequest := make(chan bool, 1)
		// create a test server which responds to our /notify call
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var data pushgateway.NotifyRequest
			if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
				t.Error(err)
			}
			notification := data.Notification
			// Validate the request
			if notification.Counts == nil {
				t.Fatal("no unread notification counts in request")
			}
			if unread := notification.Counts.Unread; unread != 1 {
				t.Errorf("expected one unread notification, got %d", unread)
			}

			if len(notification.Devices) == 0 {
				t.Fatal("expected devices in request")
			}

			// We only created one push device, so access it directly
			device := notification.Devices[0]
			if device.AppID != appID {
				t.Errorf("unexpected app_id: %s, want %s", device.AppID, appID)
			}
			if device.PushKey != pushKey {
				t.Errorf("unexpected push_key: %s, want %s", device.PushKey, pushKey)
			}

			// Return empty result, otherwise the call is handled as failed
			if _, err := w.Write([]byte("{}")); err != nil {
				t.Error(err)
			}
			close(receivedRequest)
		}))
		defer srv.Close()

		// Create DB and Dendrite base
		connStr, close := test.PrepareDBConnectionString(t, dbType)
		defer close()
		cm := sqlutil.NewConnectionManager(nil, config.DatabaseOptions{})
		db, err := storage.NewUserDatabase(ctx, cm, &config.DatabaseOptions{
			ConnectionString: config.DataSource(connStr),
		}, "test", bcrypt.MinCost, 0, "")
		if err != nil {
			t.Error(err)
		}

		// Prepare pusher with our test server URL
		if err = db.UpsertPusher(ctx, api.Pusher{
			Kind:    api.HTTPKind,
			AppID:   appID,
			PushKey: pushKey,
			Data: map[string]interface{}{
				"url": srv.URL,
			},
		}, aliceLocalpart, serverName); err != nil {
			t.Error(err)
		}

		// Insert a dummy event
		ev, err := synctypes.ToClientEvent(dummyEvent, synctypes.FormatAll, func(roomID spec.RoomID, senderID spec.SenderID) (*spec.UserID, error) {
			return queryUserIDForSender(senderID)
		})
		if err != nil {
			t.Error(err)
		}
		if err := db.InsertNotification(ctx, aliceLocalpart, serverName, dummyEvent.EventID(), 0, nil, &api.Notification{
			Event: *ev,
		}); err != nil {
			t.Error(err)
		}

		// Notify the user about a new notification
		if err := userUtil.NotifyUserCountsAsync(ctx, pushgateway.NewHTTPClient(true), aliceLocalpart, serverName, db); err != nil {
			t.Error(err)
		}
		select {
		case <-time.After(time.Second * 5):
			t.Error("timed out waiting for response")
		case <-receivedRequest:
		}
	})

}
