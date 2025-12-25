-- Expected SQL for TestSqlGeneration_Delete_OrAndOr
DELETE FROM orders
WHERE ((orders.amount = ? OR orders.id = ?) AND (orders.account_id > ? OR orders.amount < ?))

