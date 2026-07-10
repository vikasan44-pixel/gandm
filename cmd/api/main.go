package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	"gandm/internal/payment"
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

	matchingClient := matching.NewClient(cfg.MatchingServiceURL, cfg.MatchingSharedSecret)

	paymentProvider, err := payment.NewProvider(cfg.PaymentProvider)
	if err != nil {
		log.Fatalf("payment provider: %v", err)
	}

	cargoSvc := service.NewCargoService(dbPool, service.CargoServiceConfig{
		MatchRadiusCNKm:        cfg.MatchRadiusCNKm,
		MatchRadiusKZKm:        cfg.MatchRadiusKZKm,
		ContactLimitFree:       cfg.ContactLimitFree,
		ContactLimitSubscribed: cfg.ContactLimitSubscribed,
	}, matchingClient, paymentProvider)
	cargoHandler := handlers.NewCargoHandler(cargoSvc)

	warehouseSvc := service.NewWarehouseService(dbPool, s3Client)
	warehouseHandler := handlers.NewWarehouseHandler(warehouseSvc)

	antifraudSvc := service.NewAntifraudService(dbPool, s3Client)
	antifraudHandler := handlers.NewAntifraudHandler(antifraudSvc)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	// Unauthenticated credential endpoints get a per-IP limiter against
	// password brute force; /refresh shares it (token guessing).
	loginLimiter := appmiddleware.PerIPRateLimit(cfg.LoginRateLimitPerMin, time.Minute)

	r.Route("/api", func(api chi.Router) {
		api.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			if err := dbPool.Ping(r.Context()); err != nil {
				http.Error(w, `{"status":"degraded"}`, http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})

		api.With(loginLimiter).Post("/register", registerHandler.Register)
		api.With(loginLimiter).Post("/login", registerHandler.Login)
		api.With(loginLimiter).Post("/refresh", registerHandler.Refresh)

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
			protected.Get("/consolidated/{id}", cargoHandler.GetConsolidatedStatus)
			protected.Post("/consolidated/{id}/invite", cargoHandler.InviteConsolidated)
			protected.Post("/consolidated/{id}/pay", cargoHandler.PayConsolidated)
			protected.Post("/consolidated/{id}/accept", cargoHandler.AcceptConsolidated)
			protected.Post("/consolidated/{id}/select", cargoHandler.SelectConsolidatedOffer)
			protected.Post("/consolidated/{id}/offers", cargoHandler.CreateConsolidatedOffer)
			protected.Get("/consolidated/{id}/offers", cargoHandler.ListConsolidatedOffers)

			protected.Post("/ratings", cargoHandler.CreateRating)
			protected.Get("/ratings/mine", cargoHandler.ListMyReceivedRatings)
			protected.Get("/users/{id}/rating", cargoHandler.GetUserRating)

			protected.Post("/warehouse/fill-report", warehouseHandler.CreateFillReport)
			protected.Get("/warehouse/fill-reports", warehouseHandler.ListMyFillReports)
			protected.Get("/users/{id}/fill-report", warehouseHandler.GetLatestFillReport)

			protected.Get("/chats/mine", cargoHandler.ListMyChats)
			protected.Get("/chats/{id}/messages", cargoHandler.ListChatMessages)
			protected.Post("/chats/{id}/messages", cargoHandler.SendChatMessage)

			protected.Get("/routes", cargoHandler.ListMyRoutes)
			protected.Post("/routes", cargoHandler.AddMyRoute)
			protected.Delete("/routes/{id}", cargoHandler.DeleteMyRoute)

			// Автопарк (manage_fleet, ТЗ §11.1).
			protected.Get("/fleet", cargoHandler.ListMyVehicles)
			protected.Post("/fleet", cargoHandler.AddMyVehicle)
			protected.Patch("/fleet/{id}/location", cargoHandler.UpdateMyVehicleLocation)
			protected.Delete("/fleet/{id}", cargoHandler.DeleteMyVehicle)

			// Пороги отправки склада (manage_warehouse_slots, ТЗ §5.2).
			protected.Get("/dispatch-thresholds", cargoHandler.ListMyDispatchThresholds)
			protected.Put("/routes/{id}/dispatch-threshold", cargoHandler.SetRouteDispatchThreshold)
			protected.Delete("/routes/{id}/dispatch-threshold", cargoHandler.DeleteRouteDispatchThreshold)

			// Конкурс водителей (ТЗ §11.4): склад объявляет и выбирает,
			// водители ставят цены.
			protected.Post("/driver-competitions", cargoHandler.CreateDriverCompetition)
			protected.Get("/driver-competitions/mine", cargoHandler.ListMyDriverCompetitions)
			protected.Get("/driver-competitions/open", cargoHandler.ListOpenDriverCompetitions)
			protected.Post("/driver-competitions/{id}/bids", cargoHandler.CreateDriverBid)
			protected.Post("/driver-competitions/{id}/bids/{bid}/select", cargoHandler.SelectDriverBid)

			// Конкурс таможенных представителей (manage_customs_docs, ТЗ §10.2).
			protected.Get("/customs/competitions", cargoHandler.ListCustomsCompetitions)
			protected.Post("/consolidated/{id}/customs-offers", cargoHandler.CreateCustomsOffer)
			protected.Get("/consolidated/{id}/customs-offers", cargoHandler.ListCustomsOffers)
			protected.Post("/consolidated/{id}/customs-offers/{oid}/select", cargoHandler.SelectCustomsOffer)
			// Сотрудники компании (ТЗ §13.1): суб-аккаунты внутри
			// проверенного аккаунта компании.
			protected.Get("/employees", cargoHandler.ListMyEmployees)
			protected.Post("/employees", cargoHandler.CreateEmployee)
			protected.Post("/employees/{id}/block", cargoHandler.SetEmployeeBlocked)

			// Антинакрутка (ТЗ §6.2): избранное и документы сделок.
			protected.Get("/favorites", antifraudHandler.ListFavorites)
			protected.Post("/favorites", antifraudHandler.AddFavorite)
			protected.Delete("/favorites/{id}", antifraudHandler.RemoveFavorite)
			protected.Post("/deals/{id}/documents", antifraudHandler.UploadDealDocument)
			protected.Get("/deals/{id}/documents", antifraudHandler.ListDealDocuments)

			protected.Get("/notifications", cargoHandler.ListMyNotifications)
			protected.Get("/notifications/unread-count", cargoHandler.CountUnreadNotifications)
			protected.Post("/notifications/read", cargoHandler.MarkNotificationsRead)
		})

		api.Route("/admin", func(admin chi.Router) {
			admin.With(loginLimiter).Post("/login", adminHandler.Login)
			admin.With(loginLimiter).Post("/refresh", adminHandler.Refresh)

			admin.Group(func(protected chi.Router) {
				protected.Use(tokenManager.RequireAdminAuth)

				// Доступно и админу, и модератору (ТЗ §19.6: модератор —
				// верификация документов и просмотр участников).
				protected.Get("/dashboard/stats", adminHandler.DashboardStats)
				protected.Get("/audit-log", adminHandler.ListAuditLog)

				protected.Get("/verifications", adminHandler.VerificationQueue)
				protected.Get("/verifications/{id}", adminHandler.VerificationDetail)
				protected.Post("/verifications/{id}/approve", adminHandler.ApproveVerification)
				protected.Post("/verifications/{id}/reject", adminHandler.RejectVerification)

				protected.Get("/users", adminHandler.ListUsers)
				protected.Get("/users/{id}", adminHandler.GetUser)
				protected.Get("/users/{id}/routes", adminHandler.ListUserRoutes)
				protected.Get("/users/{id}/fill-reports", adminHandler.ListUserFillReports)

				// Всё, что меняет доступы/деньги/настройки — только полный
				// админ; модератор получает 403 admin_role_required.
				protected.Group(func(full chi.Router) {
					full.Use(adminHandler.RequireAdminRole)

					full.Get("/analytics", adminHandler.Analytics)
					full.Get("/suspicious", antifraudHandler.SuspiciousPairs)
					full.Get("/moderators", adminHandler.ListModerators)
					full.Post("/moderators", adminHandler.CreateModerator)

					full.Post("/users/{id}/tools", adminHandler.SetUserTools)
					full.Post("/users/{id}/apply-set", adminHandler.ApplyPermissionSet)
					full.Post("/users/{id}/block", adminHandler.BlockUser)
					full.Post("/users/{id}/unblock", adminHandler.UnblockUser)
					full.Post("/users/{id}/subscription", adminHandler.SetUserSubscription)
					full.Post("/users/{id}/routes", adminHandler.AddUserRoute)
					full.Delete("/users/{id}/routes/{routeId}", adminHandler.DeleteUserRoute)

					full.Get("/tools", adminHandler.ListTools)
					full.Post("/tools", adminHandler.CreateTool)
					full.Patch("/tools/{id}", adminHandler.UpdateTool)

					full.Get("/permission-sets", adminHandler.ListPermissionSets)
					full.Post("/permission-sets", adminHandler.CreatePermissionSet)
					full.Patch("/permission-sets/{id}", adminHandler.UpdatePermissionSet)

					full.Get("/settings", adminHandler.GetPlatformSettings)
					full.Patch("/settings", adminHandler.UpdatePlatformSettings)
					full.Post("/consolidated/{id}/payments", adminHandler.MarkConsolidatedPayment)
				})
			})
		})
	})

	server := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		// Above the 30s chi timeout so the middleware answers first with a
		// proper 504 instead of the connection just dropping.
		WriteTimeout: 65 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Graceful shutdown: stop accepting new connections on SIGINT/SIGTERM
	// and let in-flight requests (and their transactions) finish.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		log.Printf("listening on %s", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		log.Fatal(err)
	case <-ctx.Done():
		log.Println("shutting down...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("shutdown: %v", err)
		}
	}
}
