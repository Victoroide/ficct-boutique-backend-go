-- Restore the original seeded coordinates (only if still at the corrected values).

UPDATE branches
SET latitude = -17.7833, longitude = -63.1822, updated_at = NOW()
WHERE code = 'SC-01'
  AND abs(latitude - (-17.7836)) < 0.0005
  AND abs(longitude - (-63.1887)) < 0.0005;

UPDATE branches
SET latitude = -17.7900, longitude = -63.1700, updated_at = NOW()
WHERE code = 'SC-02'
  AND abs(latitude - (-17.7600)) < 0.0005
  AND abs(longitude - (-63.1950)) < 0.0005;
