package shipment

import (
	"context"
	"fmt"
	"net/smtp"
)

// Activities implements the shipment package's Activities.
// Any state shared by the worker among the activities is stored here.
type Activities struct {
	SMTPEnabled bool
	SMTPHost    string
	SMTPPort    int
}

var a Activities

// BookShipmentInput is the input for the BookShipment operation.
// All fields are required.
type BookShipmentInput struct {
	OrderID string
	Items   []Item
}

// BookShipmentResult is the result for the BookShipment operation.
// CourierReference is recorded where available, to allow tracking enquiries.
type BookShipmentResult struct {
	CourierReference string
}

// BookShipment engages a courier who can deliver the shipment to the customer
func (a *Activities) BookShipment(_ context.Context, _ *BookShipmentInput) (*BookShipmentResult, error) {
	return &BookShipmentResult{}, nil
}

const from = "orders@reference-app.example"
const to = "customer@reference-app.example"

// ShipmentBookedNotificationInput is the input for a ShipmentCreated notification.
type ShipmentBookedNotificationInput struct {
	OrderID string
}

// ShipmentBookedNotification sends a ShipmentCreated notification to a user.
func (a *Activities) ShipmentBookedNotification(_ context.Context, input *ShipmentBookedNotificationInput) error {
	err := a.sendMail(from, to,
		fmt.Sprintf("Shipment for order: %s", input.OrderID),
		"Your order has been processed and shipping has been arranged with the courier. We'll be in touch once it's dispatched.",
	)

	return err
}

// ShipmentDispatchedNotificationInput is the input for a ShipmentDispatched notification.
type ShipmentDispatchedNotificationInput struct {
	OrderID string
}

// ShipmentDispatchedNotification sends a ShipmentDispatched notification to a user.
func (a *Activities) ShipmentDispatchedNotification(_ context.Context, input *ShipmentDispatchedNotificationInput) error {
	err := a.sendMail(from, to,
		fmt.Sprintf("Shipment dispatched for order: %s", input.OrderID),
		"Your order has been dispatched.",
	)

	return err
}

// ShipmentDeliveredNotificationInput is the input for a ShipmentDelivered notification.
type ShipmentDeliveredNotificationInput struct {
	OrderID string
}

// ShipmentDeliveredNotification sends a ShipmentDelivered notification to a user.
func (a *Activities) ShipmentDeliveredNotification(_ context.Context, input *ShipmentDeliveredNotificationInput) error {
	err := a.sendMail(from, to,
		fmt.Sprintf("Shipment delivered for order: %s", input.OrderID),
		"Your order has been delivered.",
	)

	return err
}

func (a *Activities) sendMail(from string, to string, subject string, body string) error {
	if !a.SMTPEnabled {
		return nil
	}

	return smtp.SendMail(
		fmt.Sprintf("%s:%d", a.SMTPHost, a.SMTPPort),
		nil,
		from,
		[]string{to},
		[]byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s", to, subject, body)),
	)
}