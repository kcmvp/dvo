SELECT
  accounts.ID AS accounts__ID,
  profiles.ID AS profiles__ID
FROM accounts
INNER JOIN profiles ON (((accounts.ID = profiles.AccountID) AND (accounts.Email = profiles.Bio)) OR (accounts.ID = profiles.ID))
