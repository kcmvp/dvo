package xql

import (
	"context"
	"testing"

	"github.com/kcmvp/dvo/entity"
)

type Order struct {
	Id   string
	Name string
}

func (o Order) Table() string {
	return "order"
}

type OrderItem struct {
	ItemId   string
	ItemName string
	OrderId  string
}

func (o OrderItem) Table() string {
	return "order_item"
}

var _ entity.Entity = Order{}
var _ entity.Entity = OrderItem{}

var (
	OrderId          = entity.Field[Order, string]("Id")
	OrderName        = entity.Field[Order, string]("Name")
	OrderItemId      = entity.Field[OrderItem, string]("ItemId")
	OrderItemName    = entity.Field[OrderItem, string]("ItemName")
	OrderItemOrderId = entity.Field[OrderItem, string]("OrderId")
)

func TestDesign(t *testing.T) {
	// The FROM table is inferred from the first field in View()
	view := View(OrderId, OrderItemId)
	join := Join(OrderId, OrderItemOrderId)

	// The call to JoinQuery now passes the join object.
	JoinQuery(context.Background(), view, nil, join)
}
