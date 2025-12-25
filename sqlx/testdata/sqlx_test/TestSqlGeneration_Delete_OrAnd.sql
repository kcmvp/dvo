-- Expected SQL for TestSqlGeneration_Delete_OrAnd
DELETE FROM orders
WHERE ((orders.amount = ? OR orders.id = ?) AND orders.account_id > ?)

