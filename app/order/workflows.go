package order

import (
	"fmt"
	"time"

	"github.com/temporalio/orders-reference-app-go/app/billing"
	"github.com/temporalio/orders-reference-app-go/app/shipment"
	"go.temporal.io/sdk/workflow"
)

type orderImpl struct {
	id         string
	customerID string
	status     *OrderStatus
}

// Order Workflow process an order from a customer.
func Order(ctx workflow.Context, input *OrderInput) (*OrderResult, error) {
	wf := new(orderImpl)

	if err := wf.setup(ctx, input); err != nil {
		return nil, err
	}

	return wf.run(ctx, input)
}

func (o *orderImpl) setup(ctx workflow.Context, input *OrderInput) error {
	if input.ID == "" {
		return fmt.Errorf("ID is required")
	}

	if input.CustomerID == "" {
		return fmt.Errorf("CustomerID is required")
	}

	if len(input.Items) == 0 {
		return fmt.Errorf("order must contain items")
	}

	o.id = input.ID
	o.customerID = input.CustomerID
	o.status = &OrderStatus{ID: input.ID, CustomerID: input.CustomerID, Items: input.Items}

	return workflow.SetQueryHandler(ctx, StatusQuery, func() (*OrderStatus, error) {
		return o.status, nil
	})
}

func (o *orderImpl) run(ctx workflow.Context, order *OrderInput) (*OrderResult, error) {
	var result OrderResult

	fulfillments, err := o.fulfill(ctx, order.Items)
	if err != nil {
		return nil, err
	}
	o.status.Fulfillments = fulfillments

	completed := 0
	for _, f := range fulfillments {
		f := f
		workflow.Go(ctx, func(ctx workflow.Context) {
			err := o.processFulfillment(ctx, f)
			if err != nil {
				workflow.GetLogger(ctx).Error("fulfillment error", "order", order.ID, "fulfillment", f.ID, "error", err)
			}
			completed++
		})
	}

	workflow.Go(ctx, o.handleShipmentStatusUpdates)

	workflow.Await(ctx, func() bool { return completed == len(fulfillments) })

	return &result, nil
}

func (o *orderImpl) fulfill(ctx workflow.Context, items []*Item) ([]*Fulfillment, error) {
	ctx = workflow.WithActivityOptions(ctx,
		workflow.ActivityOptions{
			StartToCloseTimeout: 30 * time.Second,
		},
	)

	var result FulfillOrderResult

	err := workflow.ExecuteActivity(ctx,
		a.FulfillOrder,
		FulfillOrderInput{
			OrderID: o.id,
			Items:   items,
		},
	).Get(ctx, &result)
	if err != nil {
		return nil, err
	}

	return result.Fulfillments, nil
}

func (o *orderImpl) processFulfillment(ctx workflow.Context, fulfillment *Fulfillment) error {
	var billingItems []billing.Item
	for _, i := range fulfillment.Items {
		billingItems = append(billingItems, billing.Item{SKU: i.SKU, Quantity: i.Quantity})
	}

	var charge ChargeResult

	ctx = workflow.WithActivityOptions(ctx,
		workflow.ActivityOptions{
			StartToCloseTimeout: 30 * time.Second,
		},
	)

	f := workflow.ExecuteActivity(ctx,
		a.Charge,
		&ChargeInput{
			CustomerID: o.customerID,
			Reference:  fulfillment.ID,
			Items:      billingItems,
		},
	)
	err := f.Get(ctx, &charge)
	if err != nil {
		// TODO: Payment specific errors for business logic
		return err
	}

	shipmentID := fmt.Sprintf("Shipment:%s", fulfillment.ID)

	ctx = workflow.WithChildOptions(ctx,
		workflow.ChildWorkflowOptions{
			TaskQueue:  shipment.TaskQueue,
			WorkflowID: shipmentID,
		},
	)

	var shippingItems []shipment.Item
	for _, i := range fulfillment.Items {
		shippingItems = append(shippingItems, shipment.Item{SKU: i.SKU, Quantity: i.Quantity})
	}

	shipment := workflow.ExecuteChildWorkflow(ctx,
		shipment.Shipment,
		shipment.ShipmentInput{
			OrderID:         o.id,
			OrderWorkflowID: workflow.GetInfo(ctx).WorkflowExecution.ID,
			Items:           shippingItems,
		},
	)
	fulfillment.Shipment = &Shipment{ID: shipmentID}

	if err := shipment.Get(ctx, nil); err != nil {
		// TODO: On shipment failure, prompt user if they'd like to re-ship or cancel
		return err
	}

	return nil
}

func (o *orderImpl) handleShipmentStatusUpdates(ctx workflow.Context) {
	ch := workflow.GetSignalChannel(ctx, shipment.ShipmentStatusUpdatedSignalName)
	for {
		var signal shipment.ShipmentStatusUpdatedSignal
		_ = ch.Receive(ctx, &signal)
		for _, f := range o.status.Fulfillments {
			if f.Shipment != nil && f.Shipment.ID == signal.ShipmentID {
				f.Shipment.Status = signal.Status
				break
			}
		}
	}
}