package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"gandm/internal/auth"
	"gandm/internal/config"
	"gandm/internal/handlers"
	"gandm/internal/matching"
	appmiddleware "gandm/internal/middleware"
	"gandm/internal/repository"
	"gandm/internal/service"
	"gandm/internal/storage"
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

	dbPool, err := pgxpool.New(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(ctx); err != nil {
		log.Fatalf("db ping: %v", err)
	}

	s3Client, err := storage.NewS3Client(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3UseSSL)
	if err != nil {
		log.Fatalf("s3 client: %v", err)
	}
	if err := s3Client.EnsureBucket(ctx); err != nil {
		log.Fatalf("s3 ensure bucket: %v", err)
	}

	tokenManager := auth.NewManager(cfg.JWTAccessSecret, cfg.JWTRefreshSecret, cfg.JWTAccessTTL, cfg.JWTRefreshTTL)
	userRepo := repository.NewUserRepository(dbPool)
	toolRepo := repository.NewToolRepository(dbPool)

	registrationSvc := service.NewRegistrationService(dbPool, tokenManager, s3Client)
	registerHandler := handlers.NewRegisterHandler(registrationSvc)

	adminSvc := service.NewAdminService(dbPool, tokenManager, s3Client)
	adminHandler := handlers.NewAdminHandler(adminSvc)

	matchingClient := matching.NewClient(cfg.MatchingServiceURL)

	cargoSvc := service.NewCargoService(dbPool, service.CargoServiceConfig{
		MatchRadiusCNKm:        cfg.MatchRadiusCNKm,
		MatchRadiusKZKm:        cfg.MatchRadiusKZKm,
		ContactLimitFree:       cfg.ContactLimitFree,
		ContactLimitSubscribed: cfg.ContactLimitSubscribed,
	}, matchingClient)
	cargoHandler := handlers.NewCargoHandler(cargoSvc)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Route("/api", func(api chi.Router) {
		api.Post("/register", registerHandler.Register)
		api.Post("/login", registerHandler.Login)

		api.Group(func(protected chi.Router) {
			protected.Use(tokenManager.RequireAuth)
			protected.Use(appmiddleware.TouchLastActive(userRepo))
			protected.Post("/register/documents", registerHandler.UploadDocument)
			protected.Get("/me", registerHandler.Me)

			// Scaffolding for the tool-based access principle — see
			// handlers.ToolAccessCheck for why this route exists.
			protected.With(appmiddleware.RequireTool(toolRepo, "key")).
				Get("/tools/{key}/access-check", handlers.ToolAccessCheck)

			protected.Post("/cargo", cargoHandler.CreateCargoRequest)
			protected.Get("/cargo/mine", cargoHandler.ListMyCargoRequests)
			protected.Get("/cargo/available", cargoHandler.ListAvailableCargoRequests)
			protected.Post("/cargo/{id}/offers", cargoHandler.CreateOffer)
			protected.Get("/cargo/{id}/offers", cargoHandler.ListOffersForCargo)
			protected.Post("/cargo/{id}/select", cargoHandler.SelectOffer)

			// Consolidation (Stage 5). The static "available" segment takes
			// precedence over the {id} pattern in chi.
			protected.Get("/cargo/available/consolidated", cargoHandler.ListAvailableConsolidated)
			protected.Get("/cargo/{id}/consolidation", cargoHandler.GetConsolidation)
			protected.Post("/cargo/{id}/consolidation/{sid}/agree", cargoHandler.AgreeConsolidation)
			protected.Post("/cargo/{id}/consolidation/{sid}/decline", cargoHandler.DeclineConsolidation)
			protected.Get("/consolidated/mine", cargoHandler.ListMyConsolidated)
			protected.Post("/consolidated/{id}/offers", cargoHandler.CreateConsolidatedOffer)
			protected.Get("/consolidated/{id}/offers", cargoHandler.ListConsolidatedOffers)

			protected.Get("/chats/mine", cargoHandler.ListMyChats)
			protected.Get("/chats/{id}/messages", cargoHandler.ListChatMessages)
			protected.Post("/chats/{id}/messages", cargoHandler.SendChatMessage)

			protected.Get("/routes", cargoHandler.ListMyRoutes)
			protected.Post("/routes", cargoHandler.AddMyRoute)
			protected.Delete("/routes/{id}", cargoHandler.DeleteMyRoute)
			protected.Get("/notifications", cargoHandler.ListMyNotifications)
			protected.Post("/notifications/read", cargoHandler.MarkNotificationsRead)
		})

		api.Route("/admin", func(admin chi.Router) {
			admin.Post("/login", adminHandler.Login)

			admin.Group(func(protected chi.Router) {
				protected.Use(tokenManager.RequireAdminAuth)

				protected.Get("/dashboard/stats", adminHandler.DashboardStats)
				protected.Get("/audit-log", adminHandler.ListAuditLog)

				protected.Get("/verifications", adminHandler.VerificationQueue)
				protected.Get("/verifications/{id}", adminHandler.VerificationDetail)
				protected.Post("/verifications/{id}/approve", adminHandler.ApproveVerification)
				protected.Post("/verifications/{id}/reject", adminHandler.RejectVerification)

				protected.Get("/users", adminHandler.ListUsers)
				protected.Get("/users/{id}", adminHandler.GetUser)
				protected.Post("/users/{id}/tools", adminHandler.SetUserTools)
				protected.Post("/users/{id}/apply-set", adminHandler.ApplyPermissionSet)
				protected.Post("/users/{id}/block", adminHandler.BlockUser)
				protected.Post("/users/{id}/unblock", adminHandler.UnblockUser)
				protected.Post("/users/{id}/subscription", adminHandler.SetUserSubscription)
				protected.Get("/users/{id}/routes", adminHandler.ListUserRoutes)
				protected.Post("/users/{id}/routes", adminHandler.AddUserRoute)
				protected.Delete("/users/{id}/routes/{routeId}", adminHandler.DeleteUserRoute)

				protected.Get("/tools", adminHandler.ListTools)
				protected.Post("/tools", adminHandler.CreateTool)
				protected.Patch("/tools/{id}", adminHandler.UpdateTool)

				protected.Get("/permission-sets", adminHandler.ListPermissionSets)
				protected.Post("/permission-sets", adminHandler.CreatePermissionSet)
				protected.Patch("/permission-sets/{id}", adminHandler.UpdatePermissionSet)

				protected.Get("/settings", adminHandler.GetPlatformSettings)
				protected.Patch("/settings", adminHandler.UpdatePlatformSettings)
			})
		})
	})

	addr := ":" + cfg.ServerPort
	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
