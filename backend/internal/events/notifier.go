package events

import (
	"context"

	"github.com/nogproject/nog/backend/pkg/uuid"
)

// `WildcardTopic` indicates that a subscribed channel receives a signal for
// any `post()`.
var WildcardTopic uuid.I

// `notifier` is used by `Journal` to broadcast new event notifications via Go
// channels.
type notifier struct {
	do chan func(context.Context)
	// Key is output channel.  Value is specific topic or `WildcardTopic`.
	outputs map[chan<- uuid.I]uuid.I
}

func newNotifier() *notifier {
	return &notifier{
		do:      make(chan func(context.Context)),
		outputs: make(map[chan<- uuid.I]uuid.I),
	}
}

func (n *notifier) serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case fn := <-n.do:
			fn(ctx)
		}
	}
}

// `subscribe(ch, topic)` adds the channel `ch` to be signaled on
// `post(topic)`.  Use `subscribe(ch, WildcardTopic)` to receive a signal for
// any `post()`.
func (n *notifier) subscribe(ch chan<- uuid.I, topic uuid.I) {
	n.do <- func(context.Context) {
		n.outputs[ch] = topic
	}
}

// `unsubscribe()` removes the channel `ch`.  It blocks until `ch` has been
// removed, so that `ch` will not be spuriously signaled after `unsubscribe()`
// has returned.
func (n *notifier) unsubscribe(ch chan<- uuid.I) {
	done := make(chan struct{})
	n.do <- func(context.Context) {
		delete(n.outputs, ch)
		done <- struct{}{}
	}
	<-done
}

// `post(topic)` is an async non-blocking broadcast to the subscribed channels.
// Notifications are dropped if a subscribed channel is not ready to receive.
func (n *notifier) post(topic uuid.I) {
	n.do <- func(context.Context) {
		for ch, sel := range n.outputs {
			if sel == WildcardTopic || sel == topic {
				// Non-blocking.
				select {
				case ch <- topic:
				default:
				}
			}
		}
	}
}
