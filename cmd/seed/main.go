package main

import (
	"context"
	"fmt"
	"os"

	"github.com/ficct-boutique/backend-go/internal/auth"
	"github.com/ficct-boutique/backend-go/internal/config"
	"github.com/ficct-boutique/backend-go/internal/database"
	"github.com/ficct-boutique/backend-go/internal/models"
	"github.com/ficct-boutique/backend-go/internal/observability"
	"github.com/ficct-boutique/backend-go/internal/repository"

	"github.com/rs/zerolog/log"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}
	observability.InitLogger(cfg.LogLevel, cfg.AppEnv)

	ctx := context.Background()
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal().Err(err).Msg("connect db")
	}
	defer pool.Close()

	users := repository.NewUserRepo(pool)
	branches := repository.NewBranchRepo(pool)
	catalog := repository.NewCatalogRepo(pool)
	inv := repository.NewInventoryRepo(pool)

	// 1. Admin user
	if _, err := users.FindByEmail(ctx, "admin@ficct.local"); err != nil {
		hash, hashErr := auth.HashPassword("Admin123!")
		if hashErr != nil {
			log.Fatal().Err(hashErr).Msg("hash admin password")
		}
		if _, err := users.Create(ctx, "admin@ficct.local", hash, "Admin Boutique", models.RoleAdmin); err != nil {
			log.Fatal().Err(err).Msg("create admin")
		}
		log.Info().Msg("seeded admin user")
	}

	// 2. Customer demo user
	customerUser, err := users.FindByEmail(ctx, "cliente@ficct.local")
	if err != nil {
		hash, _ := auth.HashPassword("Cliente123!")
		customerUser, err = users.Create(ctx, "cliente@ficct.local", hash, "Maria Cliente", models.RoleCustomer)
		if err != nil {
			log.Warn().Err(err).Msg("create customer")
		}
	}
	if customerUser != nil {
		if _, err := pool.Exec(ctx, `
			INSERT INTO customers (id, user_id, full_name, phone, document_id, address)
			VALUES ($1, $1, $2, $3, $4, $5)
			ON CONFLICT (user_id) DO UPDATE
			SET full_name = EXCLUDED.full_name,
				phone = EXCLUDED.phone,
				document_id = EXCLUDED.document_id,
				address = EXCLUDED.address,
				updated_at = NOW()
		`, customerUser.ID, "Maria Cliente", "+59170000001", "CLI-001", "Santa Cruz, Bolivia"); err != nil {
			log.Warn().Err(err).Msg("seed customer profile")
		}
	}

	// 2b. Staff demo user — can view but not destructively edit
	if _, err := users.FindByEmail(ctx, "staff@ficct.local"); err != nil {
		hash, _ := auth.HashPassword("Staff123!")
		if _, err := users.Create(ctx, "staff@ficct.local", hash, "Carla Staff", models.RoleStaff); err != nil {
			log.Warn().Err(err).Msg("create staff")
		}
	}

	// 3. Branches
	existing, _ := branches.List(ctx)
	if len(existing) == 0 {
		lat1, lng1 := -17.7833, -63.1822
		lat2, lng2 := -17.7900, -63.1700
		_, _ = branches.Create(ctx, &models.Branch{Code: "SC-01", Name: "Boutique Centro", Address: "Av. Cañoto 100, Santa Cruz", Latitude: &lat1, Longitude: &lng1})
		_, _ = branches.Create(ctx, &models.Branch{Code: "SC-02", Name: "Boutique Equipetrol", Address: "Av. San Martín 200, Santa Cruz", Latitude: &lat2, Longitude: &lng2})
		log.Info().Msg("seeded branches")
	}

	// 4. Collection + products + variants
	cols, _ := catalog.ListCollections(ctx)
	if len(cols) == 0 {
		col, err := catalog.CreateCollection(ctx, "Otoño 2026", strPtr("Colección otoño-invierno 2026"), strPtr("autumn"))
		if err != nil {
			log.Fatal().Err(err).Msg("create collection")
		}

		products := []struct {
			sku, name, cat, desc string
			price                float64
		}{
			{"BLZ-001", "Blusa Seda Marfil", "blusas", "Blusa fluida de seda con corte clásico", 350},
			{"PNT-001", "Pantalón Sastre Negro", "pantalones", "Pantalón de vestir de corte recto", 480},
			{"VST-001", "Vestido Midi Floral", "vestidos", "Vestido midi con estampado floral", 620},
			{"FLD-001", "Falda Plisada Camel", "faldas", "Falda plisada en tono camel", 290},
		}
		for _, p := range products {
			imgURL := fmt.Sprintf("/static/products/%s.svg", p.sku)
			prod, err := catalog.CreateProduct(ctx, &models.Product{
				CollectionID: &col.ID,
				SKU:          p.sku,
				Name:         p.name,
				Description:  strPtr(p.desc),
				Category:     p.cat,
				BasePrice:    p.price,
				Currency:     "BOB",
				ImageURL:     strPtr(imgURL),
			})
			if err != nil {
				log.Warn().Err(err).Str("sku", p.sku).Msg("create product")
				continue
			}

			sizes := []string{"S", "M", "L"}
			colors := []string{"negro", "blanco"}
			for _, sz := range sizes {
				for _, c := range colors {
					vSKU := fmt.Sprintf("%s-%s-%s", p.sku, sz, c[:3])
					v, err := catalog.CreateVariant(ctx, &models.ProductVariant{
						ProductID: prod.ID,
						SKU:       vSKU,
						Size:      sz,
						Color:     c,
					})
					if err != nil {
						log.Warn().Err(err).Str("sku", vSKU).Msg("create variant")
						continue
					}
					for _, b := range existing {
						_, _ = inv.Upsert(ctx, v.ID, b.ID, 12, 3)
					}
					// also seed against freshly created branches
					all, _ := branches.List(ctx)
					for _, b := range all {
						_, _ = inv.Upsert(ctx, v.ID, b.ID, 12, 3)
					}
				}
			}
		}
		log.Info().Msg("seeded products, variants, inventory")
	}

	// Idempotent backfill: products created before the static-asset handler
	// existed have image_url=NULL. Point them at /static/products/<sku>.svg so
	// the admin catalog and customer app see real branded thumbnails.
	if _, err := pool.Exec(ctx, `
		UPDATE products
		SET image_url = '/static/products/' || sku || '.svg', updated_at = NOW()
		WHERE image_url IS NULL OR image_url = ''
	`); err != nil {
		log.Warn().Err(err).Msg("backfill product image_url")
	} else {
		log.Info().Msg("backfilled placeholder image URLs for legacy products")
	}

	log.Info().Msg("seed complete")
}

func strPtr(s string) *string { return &s }
