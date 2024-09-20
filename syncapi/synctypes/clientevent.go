/* Copyright 2017 Vector Creations Ltd
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package synctypes

import (
	"encoding/json"

	"github.com/neilalexander/harmony/internal/gomatrixserverlib"
	"github.com/neilalexander/harmony/internal/gomatrixserverlib/spec"
	"github.com/tidwall/gjson"
)

// PrevEventRef represents a reference to a previous event in a state event upgrade
type PrevEventRef struct {
	PrevContent   json.RawMessage `json:"prev_content"`
	ReplacesState string          `json:"replaces_state"`
	PrevSenderID  string          `json:"prev_sender"`
}

type ClientEventFormat int

const (
	// FormatAll will include all client event keys
	FormatAll ClientEventFormat = iota
	// FormatSync will include only the event keys required by the /sync API. Notably, this
	// means the 'room_id' will be missing from the events.
	FormatSync
	// FormatSyncFederation will include all event keys normally included in federated events.
	// This allows clients to request federated formatted events via the /sync API.
	FormatSyncFederation
)

// ClientFederationFields extends a ClientEvent to contain the additional fields present in a
// federation event. Used when the client requests `event_format` of type `federation`.
type ClientFederationFields struct {
	Depth      int64        `json:"depth,omitempty"`
	PrevEvents []string     `json:"prev_events,omitempty"`
	AuthEvents []string     `json:"auth_events,omitempty"`
	Signatures spec.RawJSON `json:"signatures,omitempty"`
	Hashes     spec.RawJSON `json:"hashes,omitempty"`
}

// ClientEvent is an event which is fit for consumption by clients, in accordance with the specification.
type ClientEvent struct {
	Content        spec.RawJSON   `json:"content"`
	EventID        string         `json:"event_id,omitempty"`         // EventID is omitted on receipt events
	OriginServerTS spec.Timestamp `json:"origin_server_ts,omitempty"` // OriginServerTS is omitted on receipt events
	RoomID         string         `json:"room_id,omitempty"`          // RoomID is omitted on /sync responses
	Sender         string         `json:"sender,omitempty"`           // Sender is omitted on receipt events
	SenderKey      spec.SenderID  `json:"sender_key,omitempty"`       // The SenderKey for events in pseudo ID rooms
	StateKey       *string        `json:"state_key,omitempty"`
	Type           string         `json:"type"`
	Unsigned       spec.RawJSON   `json:"unsigned,omitempty"`
	Redacts        string         `json:"redacts,omitempty"`

	// Only sent to clients when `event_format` == `federation`.
	ClientFederationFields
}

// ToClientEvents converts server events to client events.
func ToClientEvents(serverEvs []gomatrixserverlib.PDU, format ClientEventFormat) []ClientEvent {
	evs := make([]ClientEvent, 0, len(serverEvs))
	for _, se := range serverEvs {
		if se == nil {
			continue // TODO: shouldn't happen?
		}
		ev := ToClientEvent(se, format)
		evs = append(evs, *ev)
	}
	return evs
}

// ToClientEvent converts a single server event to a client event.
func ToClientEvent(se gomatrixserverlib.PDU, format ClientEventFormat) *ClientEvent {
	ce := ClientEvent{
		Content:        se.Content(),
		Sender:         string(se.SenderID()),
		Type:           se.Type(),
		StateKey:       se.StateKey(),
		Unsigned:       se.Unsigned(),
		OriginServerTS: se.OriginServerTS(),
		EventID:        se.EventID(),
		Redacts:        se.Redacts(),
	}

	switch format {
	case FormatAll:
		ce.RoomID = se.RoomID().String()
	case FormatSync:
	case FormatSyncFederation:
		ce.RoomID = se.RoomID().String()
		ce.AuthEvents = se.AuthEventIDs()
		ce.PrevEvents = se.PrevEventIDs()
		ce.Depth = se.Depth()
		// TODO: Set Signatures & Hashes fields
	}

	return &ce
}

type InviteRoomStateEvent struct {
	Content  spec.RawJSON `json:"content"`
	SenderID string       `json:"sender"`
	StateKey *string      `json:"state_key"`
	Type     string       `json:"type"`
}

func GetUpdatedInviteRoomState(userIDForSender spec.UserIDForSender, inviteRoomState gjson.Result, event gomatrixserverlib.PDU, roomID spec.RoomID, eventFormat ClientEventFormat) (spec.RawJSON, error) {
	var res spec.RawJSON
	inviteStateEvents := []InviteRoomStateEvent{}
	err := json.Unmarshal([]byte(inviteRoomState.Raw), &inviteStateEvents)
	if err != nil {
		return nil, err
	}

	res, err = json.Marshal(inviteStateEvents)
	if err != nil {
		return nil, err
	}

	return res, nil
}
