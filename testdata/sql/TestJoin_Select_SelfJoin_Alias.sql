SELECT
  accounts.ID AS accounts__ID,
  accounts.Email AS accounts__Email
FROM accounts
INNER JOIN accounts a2 ON (accounts.ID = a2.ID)
