-- Expected SQL for TestSqlGeneration_Delete_InEmpty
-- Empty IN should be translated to an always-false clause
DELETE FROM orders
WHERE 1=0

