-- Expected SQL for TestSqlGeneration_Delete_And
DELETE FROM orders
WHERE (orders.amount = ? AND orders.id > ?)

