package fclient_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/neilalexander/harmony/internal/gomatrixserverlib"
	"github.com/neilalexander/harmony/internal/gomatrixserverlib/fclient"
	"github.com/neilalexander/harmony/internal/gomatrixserverlib/spec"
	"golang.org/x/crypto/ed25519"
)

type roundTripper struct {
	fn func(*http.Request) (*http.Response, error)
}

func (r *roundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return r.fn(req)
}

// The purpose of this test is to make sure that if /v2/send_join 404s we automatically
// fallback to /v1/send_join and seamlessly handle the different response format.
func TestSendJoinFallback(t *testing.T) {
	serverName := spec.ServerName("local.server.name")
	targetServerName := spec.ServerName("target.server.name")
	keyID := gomatrixserverlib.KeyID("ed25519:auto")
	_, privateKey, _ := ed25519.GenerateKey(nil)
	roomVerImpl, _ := gomatrixserverlib.GetRoomVersion((gomatrixserverlib.RoomVersionV1))
	// we don't care about the actual contents, just that it ferries data across fine.
	retEv := spec.RawJSON(`{"auth_events":[],"content":{"creator":"@userid:baba.is.you"},"depth":0,"event_id":"$WCraVpPZe5TtHAqs:baba.is.you","hashes":{"sha256":"EehWNbKy+oDOMC0vIvYl1FekdDxMNuabXKUVzV7DG74"},"origin":"baba.is.you","origin_server_ts":0,"prev_events":[],"prev_state":[],"room_id":"!roomid:baba.is.you","sender":"@userid:baba.is.you","signatures":{"baba.is.you":{"ed25519:auto":"08aF4/bYWKrdGPFdXmZCQU6IrOE1ulpevmWBM3kiShJPAbRbZ6Awk7buWkIxlMF6kX3kb4QpbAlZfHLQgncjCw"}},"state_key":"","type":"m.room.create"}`)
	wantRes := fclient.RespSendJoin{
		StateEvents: gomatrixserverlib.EventJSONs{
			retEv,
		},
	}
	v1Res := []interface{}{
		200, wantRes,
	}
	v1ResBytes, err := json.Marshal(v1Res)
	if err != nil {
		t.Fatalf("failed to marshal RespSendJoin: %s", err)
	}
	fc := fclient.NewFederationClient(
		[]*fclient.SigningIdentity{
			{
				ServerName: serverName,
				KeyID:      keyID,
				PrivateKey: privateKey,
			},
		},
		fclient.WithSkipVerify(true),
		fclient.WithTransport(
			&roundTripper{
				fn: func(req *http.Request) (*http.Response, error) {
					if strings.HasPrefix(req.URL.Path, "/_matrix/federation/v2/send_join") {
						return &http.Response{
							StatusCode: 404,
							Body:       io.NopCloser(strings.NewReader("404 not found")),
						}, nil
					}
					if !strings.HasPrefix(req.URL.Path, "/_matrix/federation/v1/send_join") {
						return nil, fmt.Errorf("test: unexpected url path: %s", req.URL.Path)
					}
					t.Logf("Sending response: %s", string(v1ResBytes))
					return &http.Response{
						StatusCode: 200,
						Body:       io.NopCloser(bytes.NewReader(v1ResBytes)),
					}, nil
				},
			},
		),
	)
	ev, err := roomVerImpl.NewEventFromTrustedJSON(
		[]byte(`{"auth_events":[["$WCraVpPZe5TtHAqs:baba.is.you",{"sha256":"gBxQI2xzDLMoyIjkrpCJFBXC5NnrSemepc7SninSARI"}]],"content":{"membership":"join"},"depth":1,"event_id":"$fnwGrQEpiOIUoDU2:baba.is.you","hashes":{"sha256":"DqOjdFgvFQ3V/jvQW2j3ygHL4D+t7/LaIPZ/tHTDZtI"},"origin":"baba.is.you","origin_server_ts":0,"prev_events":[["$WCraVpPZe5TtHAqs:baba.is.you",{"sha256":"gBxQI2xzDLMoyIjkrpCJFBXC5NnrSemepc7SninSARI"}]],"prev_state":[],"room_id":"!roomid:baba.is.you","sender":"@userid:baba.is.you","signatures":{"baba.is.you":{"ed25519:auto":"qBWLb42zicQVsbh333YrcKpHfKokcUOM/ytldGlrgSdXqDEDDxvpcFlfadYnyvj3Z/GjA2XZkqKHanNEh575Bw"}},"state_key":"@userid:baba.is.you","type":"m.room.member"}`),
		false,
	)
	if err != nil {
		t.Fatalf("failed to read event json: %s", err)
	}
	res, err := fc.SendJoin(context.Background(), serverName, targetServerName, ev)
	if err != nil {
		t.Fatalf("SendJoin returned an error: %s", err)
	}
	if !reflect.DeepEqual(res.GetStateEvents(), wantRes.StateEvents) {
		t.Fatalf("SendJoin response got %+v want %+v", res.GetStateEvents(), wantRes.StateEvents)
	}
}

// The purpose of this test is to ensure that the federation client is capable of marshalling into
// a RespSendJoin response type. This is mostly a sanity check that EventJSON and EventJSONs behave
// correctly.
func TestSendJoinJSON(t *testing.T) {
	serverName := spec.ServerName("local.server.name")
	targetServerName := spec.ServerName("target.server.name")
	keyID := gomatrixserverlib.KeyID("ed25519:auto")
	_, privateKey, _ := ed25519.GenerateKey(nil)
	roomVerImpl, _ := gomatrixserverlib.GetRoomVersion(gomatrixserverlib.RoomVersionV1)
	// we don't care about the actual contents, just that it ferries data across fine.
	retEv := spec.RawJSON(`{"auth_events":[],"content":{"creator":"@userid:baba.is.you"},"depth":0,"event_id":"$WCraVpPZe5TtHAqs:baba.is.you","hashes":{"sha256":"EehWNbKy+oDOMC0vIvYl1FekdDxMNuabXKUVzV7DG74"},"origin":"baba.is.you","origin_server_ts":0,"prev_events":[],"prev_state":[],"room_id":"!roomid:baba.is.you","sender":"@userid:baba.is.you","signatures":{"baba.is.you":{"ed25519:auto":"08aF4/bYWKrdGPFdXmZCQU6IrOE1ulpevmWBM3kiShJPAbRbZ6Awk7buWkIxlMF6kX3kb4QpbAlZfHLQgncjCw"}},"state_key":"","type":"m.room.create"}`)
	respSendJoinResponseJSON := []byte(fmt.Sprintf(`{
		"state": [%s],
		"auth_chain": [%s]
	}`, string(retEv), string(retEv)))

	fc := fclient.NewFederationClient(
		[]*fclient.SigningIdentity{
			{
				ServerName: serverName,
				KeyID:      keyID,
				PrivateKey: privateKey,
			},
		},
		fclient.WithSkipVerify(true),
		fclient.WithTransport(
			&roundTripper{
				fn: func(req *http.Request) (*http.Response, error) {
					if strings.HasPrefix(req.URL.Path, "/_matrix/federation/v2/send_join") {
						t.Logf("Sending response: %s", string(respSendJoinResponseJSON))
						return &http.Response{
							StatusCode: 200,
							Body:       io.NopCloser(bytes.NewReader(respSendJoinResponseJSON)),
						}, nil
					}
					return &http.Response{
						StatusCode: 404,
						Body:       io.NopCloser(strings.NewReader("404 not found")),
					}, nil
				},
			},
		),
	)
	ev, err := roomVerImpl.NewEventFromTrustedJSON(
		[]byte(`{"auth_events":[["$WCraVpPZe5TtHAqs:baba.is.you",{"sha256":"gBxQI2xzDLMoyIjkrpCJFBXC5NnrSemepc7SninSARI"}]],"content":{"membership":"join"},"depth":1,"event_id":"$fnwGrQEpiOIUoDU2:baba.is.you","hashes":{"sha256":"DqOjdFgvFQ3V/jvQW2j3ygHL4D+t7/LaIPZ/tHTDZtI"},"origin":"baba.is.you","origin_server_ts":0,"prev_events":[["$WCraVpPZe5TtHAqs:baba.is.you",{"sha256":"gBxQI2xzDLMoyIjkrpCJFBXC5NnrSemepc7SninSARI"}]],"prev_state":[],"room_id":"!roomid:baba.is.you","sender":"@userid:baba.is.you","signatures":{"baba.is.you":{"ed25519:auto":"qBWLb42zicQVsbh333YrcKpHfKokcUOM/ytldGlrgSdXqDEDDxvpcFlfadYnyvj3Z/GjA2XZkqKHanNEh575Bw"}},"state_key":"@userid:baba.is.you","type":"m.room.member"}`),
		false,
	)
	if err != nil {
		t.Fatalf("failed to read event json: %s", err)
	}
	t.Logf("%s - %#v", string(ev.JSON()), ev)
	res, err := fc.SendJoin(context.Background(), serverName, targetServerName, ev)
	if err != nil {
		t.Fatalf("SendJoin returned an error: %s", err)
	}
	wantStateEvents := gomatrixserverlib.EventJSONs{[]byte(retEv)}
	wantAuthChain := gomatrixserverlib.EventJSONs{[]byte(retEv)}
	if !reflect.DeepEqual(res.GetStateEvents(), wantStateEvents) {
		t.Fatalf("SendJoin response got state %+v want %+v", jsonify(res.GetStateEvents()), jsonify(wantStateEvents))
	}
	if !reflect.DeepEqual(res.GetAuthEvents(), wantAuthChain) {
		t.Fatalf("SendJoin response got auth %+v want %+v", jsonify(res.GetAuthEvents()), jsonify(wantAuthChain))
	}
}

func jsonify(x interface{}) string {
	b, _ := json.Marshal(x)
	return string(b)
}
