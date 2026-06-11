-- Correct misplaced seeded branch coordinates so map distances match the
-- printed addresses. Guarded by the old values: rows that were manually
-- relocated since seeding are left untouched.

-- SC-01 Boutique Centro, Av. Cañoto 100 (primer anillo oeste). The old value
-- pointed at Plaza 24 de Septiembre, ~700 m east of Av. Cañoto.
UPDATE branches
SET latitude = -17.7836, longitude = -63.1887, updated_at = NOW()
WHERE code = 'SC-01'
  AND abs(latitude - (-17.7833)) < 0.0005
  AND abs(longitude - (-63.1822)) < 0.0005;

-- SC-02 Boutique Equipetrol, Av. San Martín 200. The old value sat ~4.5 km
-- southeast of the actual Equipetrol district.
UPDATE branches
SET latitude = -17.7600, longitude = -63.1950, updated_at = NOW()
WHERE code = 'SC-02'
  AND abs(latitude - (-17.7900)) < 0.0005
  AND abs(longitude - (-63.1700)) < 0.0005;
