package external

import (
	"context"

	"github.com/neilalexander/harmony/roomserver/api"
	"github.com/neilalexander/harmony/setup/jetstream"
)

type RoomserverAPIServer struct {
	nats *jetstream.NATSInstance
	api.RoomserverInternalAPI
}

func NewRoomserverAPIServer(intapi api.RoomserverInternalAPI, nats *jetstream.NATSInstance) (*RoomserverAPIServer, error) {
	s := &RoomserverAPIServer{
		RoomserverInternalAPI: intapi,
		nats:                  nats,
	}
	ctx := context.TODO()

	if err := jetstream.ListenAPI(
		s.nats, "Roomserver", "PerformBackfill",
		func(req *api.PerformBackfillRequest, res *api.PerformBackfillResponse) error {
			return s.RoomserverInternalAPI.PerformBackfill(ctx, req, res)
		},
	); err != nil {
		return nil, err
	}

	return s, nil
}
