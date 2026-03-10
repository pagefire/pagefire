package notification

import (
	"context"
	"strings"
	"testing"

	"github.com/pagefire/pagefire/internal/store"
)

type mockProvider struct {
	typeName string
	sendFunc func(ctx context.Context, msg Message) (string, error)
}

func (m *mockProvider) Type() string { return m.typeName }

func (m *mockProvider) Send(ctx context.Context, msg Message) (string, error) {
	return m.sendFunc(ctx, msg)
}

func (m *mockProvider) ValidateTarget(target string) error { return nil }

func TestDispatcher_DispatchToRegisteredProvider(t *testing.T) {
	d := NewDispatcher()

	var called bool
	d.Register(&mockProvider{
		typeName: "email",
		sendFunc: func(ctx context.Context, msg Message) (string, error) {
			called = true
			if msg.To != "alice@example.com" {
				t.Errorf("expected To=alice@example.com, got %s", msg.To)
			}
			if msg.Subject != "Alert fired" {
				t.Errorf("expected Subject=Alert fired, got %s", msg.Subject)
			}
			return "msg-001", nil
		},
	})

	n := store.Notification{
		DestinationType: "email",
		Destination:     "alice@example.com",
		Subject:         "Alert fired",
		Body:            "Something broke",
		AlertID:         "alert-1",
	}

	providerID, err := d.Dispatch(context.Background(), n)
	if err != nil {
		t.Fatalf("Dispatch failed: %v", err)
	}
	if !called {
		t.Error("expected provider Send to be called")
	}
	if providerID != "msg-001" {
		t.Errorf("expected providerID=msg-001, got %s", providerID)
	}
}

func TestDispatcher_DispatchToUnregisteredType(t *testing.T) {
	d := NewDispatcher()

	n := store.Notification{
		DestinationType: "sms",
		Destination:     "+15551234567",
		Subject:         "Alert",
		Body:            "body",
	}

	_, err := d.Dispatch(context.Background(), n)
	if err == nil {
		t.Fatal("expected error for unregistered provider type")
	}
	if !strings.Contains(err.Error(), "sms") {
		t.Errorf("error should mention the missing type, got: %v", err)
	}
}

func TestDispatcher_RegisterMultipleProviders(t *testing.T) {
	d := NewDispatcher()

	var emailCalled, smsCalled bool

	d.Register(&mockProvider{
		typeName: "email",
		sendFunc: func(ctx context.Context, msg Message) (string, error) {
			emailCalled = true
			return "email-001", nil
		},
	})
	d.Register(&mockProvider{
		typeName: "sms",
		sendFunc: func(ctx context.Context, msg Message) (string, error) {
			smsCalled = true
			return "sms-001", nil
		},
	})

	// Dispatch to SMS
	smsNotification := store.Notification{
		DestinationType: "sms",
		Destination:     "+15551234567",
		Subject:         "Alert",
		Body:            "body",
	}
	providerID, err := d.Dispatch(context.Background(), smsNotification)
	if err != nil {
		t.Fatalf("Dispatch to sms failed: %v", err)
	}
	if !smsCalled {
		t.Error("expected sms provider to be called")
	}
	if emailCalled {
		t.Error("email provider should not have been called for sms dispatch")
	}
	if providerID != "sms-001" {
		t.Errorf("expected providerID=sms-001, got %s", providerID)
	}

	// Dispatch to email
	emailNotification := store.Notification{
		DestinationType: "email",
		Destination:     "bob@example.com",
		Subject:         "Alert",
		Body:            "body",
	}
	providerID, err = d.Dispatch(context.Background(), emailNotification)
	if err != nil {
		t.Fatalf("Dispatch to email failed: %v", err)
	}
	if !emailCalled {
		t.Error("expected email provider to be called")
	}
	if providerID != "email-001" {
		t.Errorf("expected providerID=email-001, got %s", providerID)
	}
}
