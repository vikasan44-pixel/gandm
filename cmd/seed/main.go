// Command seed sets up baseline data for local development and the smoke
// test: one admin, a handful of placeholder tools, a few permission-set
// presets built from those tools, and five test participants covering every
// user status. Safe to run more than once — every step is get-or-create /
// upsert.
//
// The concrete tool keys and preset names below are illustrative
// placeholders for exercising the tools/permission_sets machinery; they are
// not a finalized business taxonomy for the freight platform. Real tool
// definitions arrive with the features that need them (Stage 2+).
//
// Usage: go run ./cmd/seed
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"gandm/internal/config"
	"gandm/internal/models"
	"gandm/internal/repository"
)

const (
	seedAdminEmail    = "admin@platform.local"
	seedAdminPassword = "Admin12345!"
	seedUserPassword  = "Test12345!"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env not found, relying on process environment")
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	db, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer db.Close()

	admin, err := seedAdmin(ctx, db)
	if err != nil {
		log.Fatalf("seed admin: %v", err)
	}
	log.Printf("admin ready: %s / %s", seedAdminEmail, seedAdminPassword)

	toolIDs, err := seedTools(ctx, db)
	if err != nil {
		log.Fatalf("seed tools: %v", err)
	}
	log.Printf("seeded %d tools", len(toolIDs))

	if err := seedPermissionSets(ctx, db, toolIDs); err != nil {
		log.Fatalf("seed permission sets: %v", err)
	}
	log.Printf("seeded %d permission sets", len(baseSets))

	if err := seedParticipants(ctx, db, admin.ID, toolIDs); err != nil {
		log.Fatalf("seed participants: %v", err)
	}
	log.Printf("seeded %d test participants (password: %s)", len(baseParticipants), seedUserPassword)

	log.Println("seed complete")
}

func seedAdmin(ctx context.Context, db *pgxpool.Pool) (*models.Admin, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(seedAdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	const q = `
		INSERT INTO admins (id, email, password_hash, role, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email) DO UPDATE SET password_hash = EXCLUDED.password_hash
		RETURNING id, email, password_hash, role, created_at
	`
	var a models.Admin
	err = db.QueryRow(ctx, q, uuid.New(), seedAdminEmail, string(hash), models.AdminRoleAdmin, time.Now()).
		Scan(&a.ID, &a.Email, &a.PasswordHash, &a.Role, &a.CreatedAt)
	return &a, err
}

type toolSpec struct {
	key, name, description, category string
}

// create_cargo_request and view_cargo_requests were seeded in Stage 1 as
// placeholders for "the future cargo module". Stage 2 arrived with its own
// tools (receive_cargo_by_route, submit_offer) that actually gate the real
// endpoints — submitting a cargo request turned out to be a free action for
// any eligible user, not tool-gated, so create_cargo_request in particular
// is now unused. Left in place rather than deleted (Stage 1 data, not ours
// to recreate) — deactivate it from the admin Tools screen if it's just
// noise.
var baseTools = []toolSpec{
	{"verify_participants", "Верификация участников", "Просмотр очереди верификации, одобрение и отклонение заявок", "admin"},
	{"manage_tools", "Управление инструментами и наборами", "Создание инструментов, сборка наборов прав, назначение участникам", "admin"},
	{"view_users", "Просмотр участников", "Просмотр списка и карточек участников платформы", "admin"},
	{"create_cargo_request", "Создание заявки на груз", "Placeholder для будущего модуля заявок (Этап 2+)", "cargo"},
	{"view_cargo_requests", "Просмотр заявок на груз", "Placeholder для будущего модуля заявок (Этап 2+)", "cargo"},
	{"manage_warehouse_slots", "Управление складскими местами", "Placeholder для будущего складского модуля", "warehouse"},
	{"manage_fleet", "Управление автопарком", "Placeholder для будущего модуля перевозчика", "carrier"},
	{"manage_customs_docs", "Работа с таможенными документами", "Placeholder для будущего таможенного модуля", "customs"},
	{"receive_cargo_by_route", "Получение заявок на груз", "Открывает GET /api/cargo/available. Пока без фильтра по направлению — хранить маршруты участника негде (Этап 2)", "cargo"},
	{"submit_offer", "Подача предложений на грузы", "Открывает POST /api/cargo/:id/offers (Этап 2)", "cargo"},
	{"submit_fill_report", "Отчёты о заполняемости склада", "Открывает POST /api/warehouse/fill-report (Этап 6)", "warehouse"},
}

func seedTools(ctx context.Context, db *pgxpool.Pool) (map[string]uuid.UUID, error) {
	toolRepo := repository.NewToolRepository(db)
	ids := make(map[string]uuid.UUID, len(baseTools))
	for _, spec := range baseTools {
		t := &models.Tool{
			ID:          uuid.New(),
			Key:         spec.key,
			Name:        spec.name,
			Description: spec.description,
			Category:    spec.category,
			IsActive:    true,
		}
		if err := toolRepo.UpsertByKey(ctx, t); err != nil {
			return nil, fmt.Errorf("tool %s: %w", spec.key, err)
		}
		ids[spec.key] = t.ID
	}
	return ids, nil
}

type setSpec struct {
	name, description string
	toolKeys           []string
}

var baseSets = []setSpec{
	{"Базовый клиент", "Стартовый доступ для клиента: создание и просмотр заявок на груз", []string{"create_cargo_request", "view_cargo_requests"}},
	{"Складской оператор", "Доступ для склада: складские места, заявки на груз, предложения и отчёты о заполняемости", []string{"manage_warehouse_slots", "view_cargo_requests", "receive_cargo_by_route", "submit_offer", "submit_fill_report"}},
	{"Перевозчик", "Доступ для перевозчика: автопарк, заявки на груз и подача предложений", []string{"manage_fleet", "view_cargo_requests", "receive_cargo_by_route", "submit_offer"}},
}

func seedPermissionSets(ctx context.Context, db *pgxpool.Pool, toolIDs map[string]uuid.UUID) error {
	setRepo := repository.NewPermissionSetRepository(db)

	for _, spec := range baseSets {
		set, err := setRepo.GetByName(ctx, spec.name)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			set = &models.PermissionSet{ID: uuid.New(), Name: spec.name, Description: spec.description}
			if err := setRepo.Create(ctx, set); err != nil {
				return fmt.Errorf("create set %s: %w", spec.name, err)
			}
		case err != nil:
			return fmt.Errorf("lookup set %s: %w", spec.name, err)
		default:
			set.Description = spec.description
			if err := setRepo.Update(ctx, set); err != nil {
				return fmt.Errorf("update set %s: %w", spec.name, err)
			}
		}

		ids, err := resolveToolIDs(toolIDs, spec.toolKeys)
		if err != nil {
			return fmt.Errorf("set %s: %w", spec.name, err)
		}
		if err := setRepo.ReplaceSetTools(ctx, set.ID, ids); err != nil {
			return fmt.Errorf("assign tools to set %s: %w", spec.name, err)
		}
	}
	return nil
}

// Reference WGS-84 coordinates for the seed routes. Almaty→Urumqi matches
// the cargo request smoke.sh submits, so radius matching has a ready
// positive case.
var (
	pointAlmaty  = models.GeoPoint{Lat: 43.238949, Lng: 76.889709, Label: "Алматы", Source: models.CoordSourceOSM, Country: "kz"}
	pointUrumqi  = models.GeoPoint{Lat: 43.825592, Lng: 87.616848, Label: "Урумчи", Source: models.CoordSourceOSM, Country: "cn"}
	pointKhorgos = models.GeoPoint{Lat: 44.2107, Lng: 80.4184, Label: "Хоргос", Source: models.CoordSourceOSM, Country: "cn"}
)

type routeSpec struct {
	origin, destination models.GeoPoint
}

type participantSpec struct {
	email           string
	companyName     string
	participantType models.ParticipantType
	status          models.UserStatus
	rejectReason    string
	toolKeys        []string
	routes          []routeSpec
}

// warehouse.active gets Алматы→Урумчи on purpose: it matches the cargo
// request smoke.sh submits, so route-based visibility has a ready positive
// case. carrier.blocked gets a route too (spec: 1-2 routes for
// warehouses/carriers) but is blocked, so it must never be notified —
// a built-in negative case.
var baseParticipants = []participantSpec{
	{"client.pending@example.com", "ООО Клиент Ожидающий", models.ParticipantClient, models.UserStatusPending, "", nil, nil},
	{"client.active@example.com", "ООО Клиент Активный", models.ParticipantClient, models.UserStatusActive, "", []string{"create_cargo_request", "view_cargo_requests"}, nil},
	{"warehouse.active@example.com", "Склад Восток", models.ParticipantWarehouse, models.UserStatusActive, "", []string{"manage_warehouse_slots", "receive_cargo_by_route", "submit_offer", "submit_fill_report"}, []routeSpec{{pointAlmaty, pointUrumqi}, {pointUrumqi, pointAlmaty}}},
	{"carrier.blocked@example.com", "ИП Перевозчик Заблокированный", models.ParticipantCarrier, models.UserStatusBlocked, "", nil, []routeSpec{{pointKhorgos, pointAlmaty}}},
	{"broker.rejected@example.com", "Брокер Отклонённый", models.ParticipantBroker, models.UserStatusRejected, "Документы не соответствуют требованиям", nil, nil},
}

func seedParticipants(ctx context.Context, db *pgxpool.Pool, adminID uuid.UUID, toolIDs map[string]uuid.UUID) error {
	userRepo := repository.NewUserRepository(db)
	verRepo := repository.NewVerificationRepository(db)
	toolRepo := repository.NewToolRepository(db)
	routeRepo := repository.NewParticipantRouteRepository(db)

	hash, err := bcrypt.GenerateFromPassword([]byte(seedUserPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	for _, spec := range baseParticipants {
		user, err := userRepo.GetByEmail(ctx, spec.email)
		switch {
		case errors.Is(err, repository.ErrNotFound):
			now := time.Now()
			user = &models.User{
				ID:              uuid.New(),
				Email:           spec.email,
				Phone:           "+70000000000",
				CompanyName:     spec.companyName,
				ParticipantType: spec.participantType,
				PasswordHash:    string(hash),
				Status:          models.UserStatusPending,
				Language:        "ru",
				CreatedAt:       now,
			}
			if err := userRepo.Create(ctx, user); err != nil {
				return fmt.Errorf("create user %s: %w", spec.email, err)
			}

			verification := &models.VerificationRequest{ID: uuid.New(), UserID: user.ID, Status: models.VerificationPending, CreatedAt: now}
			if err := verRepo.Create(ctx, verification); err != nil {
				return fmt.Errorf("create verification for %s: %w", spec.email, err)
			}

			if spec.status != models.UserStatusPending {
				if err := applySeedStatus(ctx, userRepo, verRepo, user, verification, spec, adminID); err != nil {
					return fmt.Errorf("apply status for %s: %w", spec.email, err)
				}
			}
		case err != nil:
			return fmt.Errorf("lookup user %s: %w", spec.email, err)
		default:
			if err := userRepo.UpdateStatus(ctx, user.ID, spec.status); err != nil {
				return fmt.Errorf("update status for %s: %w", spec.email, err)
			}
		}

		if len(spec.toolKeys) > 0 {
			ids, err := resolveToolIDs(toolIDs, spec.toolKeys)
			if err != nil {
				return fmt.Errorf("participant %s: %w", spec.email, err)
			}
			if err := toolRepo.ReplaceUserTools(ctx, user.ID, ids); err != nil {
				return fmt.Errorf("assign tools to %s: %w", spec.email, err)
			}
		}

		for _, rt := range spec.routes {
			route := &models.ParticipantRoute{
				ID:          uuid.New(),
				UserID:      user.ID,
				Origin:      rt.origin,
				Destination: rt.destination,
				CreatedAt:   time.Now(),
			}
			err := routeRepo.Create(ctx, route)
			if err != nil && !errors.Is(err, repository.ErrRouteExists) {
				return fmt.Errorf("add route %s→%s to %s: %w", rt.origin.Label, rt.destination.Label, spec.email, err)
			}
		}
	}
	return nil
}

// applySeedStatus fast-forwards a freshly created (pending) test user to the
// status the spec wants, writing a matching verification_request outcome so
// the two stay consistent (mirrors what the real approve/reject flow does).
func applySeedStatus(ctx context.Context, userRepo *repository.UserRepository, verRepo *repository.VerificationRepository, user *models.User, verification *models.VerificationRequest, spec participantSpec, adminID uuid.UUID) error {
	now := time.Now()
	switch spec.status {
	case models.UserStatusActive, models.UserStatusBlocked:
		if err := verRepo.UpdateStatus(ctx, verification.ID, models.VerificationApproved, nil, adminID, now); err != nil {
			return err
		}
	case models.UserStatusRejected:
		reason := spec.rejectReason
		if err := verRepo.UpdateStatus(ctx, verification.ID, models.VerificationRejected, &reason, adminID, now); err != nil {
			return err
		}
	}
	return userRepo.UpdateStatus(ctx, user.ID, spec.status)
}

func resolveToolIDs(toolIDs map[string]uuid.UUID, keys []string) ([]uuid.UUID, error) {
	ids := make([]uuid.UUID, 0, len(keys))
	for _, key := range keys {
		id, ok := toolIDs[key]
		if !ok {
			return nil, fmt.Errorf("unknown tool key %q", key)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
