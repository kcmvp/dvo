SELECT
  orders.ID AS orders__ID,
  orders.AccountID AS orders__AccountID,
  orders.Amount AS orders__Amount,
  orders.CreatedAt AS orders__CreatedAt,
  orders.UpdatedAt AS orders__UpdatedAt,
  orders.CreatedBy AS orders__CreatedBy,
  orders.UpdatedBy AS orders__UpdatedBy
FROM orders
WHERE orders.ID IN (?,?,?)
