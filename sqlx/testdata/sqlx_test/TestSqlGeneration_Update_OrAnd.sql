-- Expected SQL for TestSqlGeneration_Update_OrAnd
UPDATE orders SET orders.id = ?, orders.account_id = ?, orders.amount = ?, orders.created_at = ?, orders.updated_at = ?, orders.created_by = ?, orders.updated_by = ?
WHERE ((orders.amount = ? OR orders.id = ?) AND orders.account_id > ?)

