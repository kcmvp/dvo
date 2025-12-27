SELECT
  profiles.ID AS profiles__ID,
  profiles.AccountID AS profiles__AccountID,
  profiles.Bio AS profiles__Bio,
  profiles.Birthday AS profiles__Birthday,
  profiles.CreatedAt AS profiles__CreatedAt,
  profiles.UpdatedAt AS profiles__UpdatedAt,
  profiles.CreatedBy AS profiles__CreatedBy,
  profiles.UpdatedBy AS profiles__UpdatedBy
FROM profiles
WHERE (profiles.AccountID = ? AND profiles.Bio LIKE ?)
