SELECT
  order_items.ID AS order_items__ID,
  order_items.OrderID AS order_items__OrderID,
  order_items.ProductID AS order_items__ProductID,
  order_items.Quantity AS order_items__Quantity,
  order_items.UnitPrice AS order_items__UnitPrice,
  order_items.CreatedAt AS order_items__CreatedAt,
  order_items.UpdatedAt AS order_items__UpdatedAt,
  order_items.CreatedBy AS order_items__CreatedBy,
  order_items.UpdatedBy AS order_items__UpdatedBy
FROM order_items
WHERE (order_items.OrderID = ? AND order_items.Quantity > ?)
