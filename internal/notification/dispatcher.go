package notification

import (
	"context"
	"fmt"

	"github.com/pagefire/pagefire/internal/store"
)

// Message is the payload sent to a notification provider.
type Message struct {
	To      string
	Subject string
	Body    string
	AlertID string
}

// Provider is the interface for notification delivery backends.
type Provider interface {
	Type() string
	Send(ctx context.Context, msg Message) (providerID string, err error)
	ValidateTarget(target string) error
}

// Dispatcher routes notifications to the correct provider.
type Dispatcher struct {
	providers map[string]Provider
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{providers: make(map[string]Provider)}
}

func (d *Dispatcher) Register(p Provider) {
	d.providers[p.Type()] = p
}

// Dispatch sends a notification via the appropriate provider.
func (d *Dispatcher) Dispatch(ctx context.Context, n store.Notification) (string, error) {
	p, ok := d.providers[n.DestinationType]
	if !ok {
		return "", fmt.Errorf("no provider registered for type %q", n.DestinationType)
	}

	msg := Message{
		To:      n.Destination,
		Subject: n.Subject,
		Body:    n.Body,
		AlertID: n.AlertID,
	}

	return p.Send(ctx, msg)
}
