-- Expected SQL for TestSqlGeneration_Delete_Or
DELETE FROM orders
WHERE (orders.amount = ? OR orders.id = ?)

