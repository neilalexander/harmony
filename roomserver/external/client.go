package external

import (
	"github.com/neilalexander/harmony/roomserver/api"
	"github.com/neilalexander/harmony/setup/jetstream"
)

type RoomserverAPIClient struct {
	nats *jetstream.NATSInstance
	api.RoomserverInternalAPI
}

func NewRoomserverAPIClient(intapi api.RoomserverInternalAPI, nats *jetstream.NATSInstance) *RoomserverAPIClient {
	c := &RoomserverAPIClient{
		RoomserverInternalAPI: intapi,
		nats:                  nats,
	}
	return c
}

/*
func (c *RoomserverAPIClient) PerformBackfill(
	ctx context.Context,
	req *api.PerformBackfillRequest,
	res *api.PerformBackfillResponse,
) error {
	return jetstream.CallAPI(c.nats, "Roomserver", "PerformBackfill", req, res)
}
*/
