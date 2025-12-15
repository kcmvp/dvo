package sqlx

import (
	"fmt"
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

func TestJoinQuery(t *testing.T) {
	//view := View(OrderId, OrderItemId)
	//join := Join(OrderId, OrderItemOrderId)
	//JoinQuery(context.Background(), view, nil, join)
	fmt.Printf("waiting to implement...\n")
}

func TestJoinDelete(t *testing.T) {
	//join := Join(OrderId, OrderItemOrderId)
	//JoinDelete[Order](context.Background(), nil, join)
	fmt.Printf("TestJoinDelete to be implemented")
}

func TestJoinUpdate(t *testing.T) {
	//join := Join(OrderId, OrderItemOrderId)
	//var setter ValueObject[Order]
	//// The where clause can now be multi-table.
	//// The target table for update is inferred from the setter's type.
	//JoinUpdate(context.Background(), nil, []Joint{join}, setter)
	fmt.Printf("TestJoinUpdate to be implemented")
}
