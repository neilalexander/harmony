package jetstream

import (
	"context"
	"errors"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/sirupsen/logrus"
)

// JetStreamConsumer starts a durable consumer on the given subject with the
// given durable name. The function will be called when one or more messages
// is available, up to the maximum batch size specified. If the batch is set to
// 1 then messages will be delivered one at a time. If the function is called,
// the messages array is guaranteed to be at least 1 in size. Any provided NATS
// options will be passed through to the pull subscriber creation. The consumer
// will continue to run until the context expires, at which point it will stop.
func JetStreamConsumer(
	ctx context.Context, js nats.JetStreamContext, subj, durable string, batch int,
	f func(ctx context.Context, msgs []*nats.Msg) bool,
	opts ...nats.SubOpt,
) error {
	// Hangover from the migration from push consumers to pull consumers.
	durable = durable + "Pull"
	sub, err := js.PullSubscribe(subj, durable, opts...)
	if err != nil {
		logrus.WithContext(ctx).WithError(err).Warnf("Failed to configure durable %q", durable)
		return err
	}
	go jetStreamConsumerWorker(ctx, sub, subj, batch, f)
	return nil
}

func jetStreamConsumerWorker(
	ctx context.Context, sub *nats.Subscription, subj string, batch int,
	f func(ctx context.Context, msgs []*nats.Msg) bool,
) {
	for {
		// If the parent context has given up then there's no point in
		// carrying on doing anything, so stop the listener.
		select {
		case <-ctx.Done():
			return
		default:
		}
		// The context behaviour here is surprising — we supply a context
		// so that we can interrupt the fetch if we want, but NATS will still
		// enforce its own deadline (roughly 5 seconds by default). Therefore
		// it is our responsibility to check whether our context expired or
		// not when a context error is returned. Footguns. Footguns everywhere.
		msgs, err := sub.Fetch(batch, nats.Context(ctx))
		if err != nil {
			if err == context.Canceled || err == context.DeadlineExceeded {
				// Work out whether it was the JetStream context that expired
				// or whether it was our supplied context.
				select {
				case <-ctx.Done():
					// The supplied context expired, so we want to stop the
					// consumer altogether.
					return
				default:
					// The JetStream context expired, so the fetch probably
					// just timed out and we should try again.
					continue
				}
			} else if errors.Is(err, nats.ErrTimeout) {
				// Pull request was invalidated, try again.
				continue
			} else if errors.Is(err, nats.ErrConsumerLeadershipChanged) {
				// Leadership changed so pending pull requests became invalidated,
				// just try again.
				continue
			} else if err.Error() == "nats: Server Shutdown" {
				// The server is shutting down, but we'll rely on reconnect
				// behaviour to try and either connect us to another node (if
				// clustered) or to reconnect when the server comes back up.
				continue
			} else {
				// Something else went wrong.
				logrus.WithContext(ctx).WithField("subject", subj).WithError(err).Warn("Error on pull subscriber fetch")
				return
			}
		}
		if len(msgs) < 1 {
			continue
		}
		for _, msg := range msgs {
			if err = msg.InProgress(nats.Context(ctx)); err != nil {
				logrus.WithContext(ctx).WithField("subject", subj).Warn(fmt.Errorf("msg.InProgress: %w", err))
				continue
			}
		}
		if f(ctx, msgs) {
			for _, msg := range msgs {
				if err = msg.AckSync(nats.Context(ctx)); err != nil {
					logrus.WithContext(ctx).WithField("subject", subj).Warn(fmt.Errorf("msg.AckSync: %w", err))
				}
			}
		} else {
			for _, msg := range msgs {
				if err = msg.Nak(nats.Context(ctx)); err != nil {
					logrus.WithContext(ctx).WithField("subject", subj).Warn(fmt.Errorf("msg.Nak: %w", err))
				}
			}
		}
	}
}
