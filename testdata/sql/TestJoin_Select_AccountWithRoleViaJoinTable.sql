SELECT
  accounts.ID AS accounts__ID,
  accounts.Email AS accounts__Email,
  roles.ID AS roles__ID,
  roles.Key AS roles__Key
FROM accounts
INNER JOIN roles ON (accounts.ID = roles.ID)
WHERE ((account_roles.RoleID = roles.ID) AND (accounts.ID = roles.ID))
