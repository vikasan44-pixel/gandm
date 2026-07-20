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
	activeSessionRepo := repository.NewActiveSessionRepository(dbPool)
	tokenManager.SetSessionValidator(activeSessionRepo.IsActive)

	registrationSvc := service.NewRegistrationService(dbPool, tokenManager, s3Client)
	registerHandler := handlers.NewRegisterHandler(registrationSvc, cfg.CookieSecure)

	adminSvc := service.NewAdminService(dbPool, tokenManager, s3Client)
	adminHandler := handlers.NewAdminHandler(adminSvc, cfg.CookieSecure)

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
		DefaultCurrency:        cfg.DefaultCurrency,
	}, matchingClient, paymentProvider, s3Client)
	cargoHandler := handlers.NewCargoHandler(cargoSvc)

	warehouseSvc := service.NewWarehouseService(dbPool, s3Client)
	warehouseHandler := handlers.NewWarehouseHandler(warehouseSvc)

	antifraudSvc := service.NewAntifraudService(dbPool, s3Client)
	antifraudHandler := handlers.NewAntifraudHandler(antifraudSvc)

	ratesSvc := service.NewRatesService(dbPool)
	ratesHandler := handlers.NewRatesHandler(ratesSvc)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(appmiddleware.LimitJSONBody(1 << 20)) // 1 MiB; uploads have their own limit.
	// Stop browsers from MIME-sniffing responses into a different content type.
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			next.ServeHTTP(w, req)
		})
	})
	r.Use(middleware.Timeout(30 * time.Second))

	// Unauthenticated credential endpoints get a per-IP limiter against
	// password brute force; /refresh shares it (token guessing). Счётчики
	// в Postgres — лимит держится и при нескольких инстансах бэкенда.
	loginLimiter := appmiddleware.PerIPRateLimitDB(dbPool, cfg.LoginRateLimitPerMin, time.Minute)

	r.Route("/api", func(api chi.Router) {
		api.Get("/health", func(w http.ResponseWriter, r *http.Request) {
			if err := dbPool.Ping(r.Context()); err != nil {
				http.Error(w, `{"status":"degraded"}`, http.StatusServiceUnavailable)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})

		// Каталог участнических инструментов — публичный: нужен на экране
		// регистрации, где человек ещё не авторизован.
		api.Get("/tools/catalog", cargoHandler.ToolCatalog)

		// Гостевой поиск (без авторизации): анонимные карточки грузов и
		// транспорта по координатам+радиусу. Контакты и детали — только после
		// входа. Регистрируем ДО protected-группы.
		api.Get("/public/cargo", cargoHandler.PublicSearchCargo)
		api.Get("/public/transport", cargoHandler.PublicSearchTransport)

		// Официальные курсы НБРК — публичные справочные данные для
		// приблизительной подсказки «≈ в вашей валюте». На суммы сделок не влияют.
		api.Get("/currency-rates", ratesHandler.CurrencyRates)

		api.With(loginLimiter).Post("/register", registerHandler.Register)
		api.With(loginLimiter).Post("/login", registerHandler.Login)
		api.With(loginLimiter).Post("/refresh", registerHandler.Refresh)
		// Logout is cookie-based (revokes the refresh token) — no access token
		// required, so it lives outside the protected group.
		api.Post("/logout", registerHandler.Logout)

		api.Group(func(protected chi.Router) {
			protected.Use(tokenManager.RequireAuth)
			protected.Use(appmiddleware.TouchLastActive(userRepo))
			protected.Post("/register/documents", registerHandler.UploadDocument)
			protected.Get("/me", registerHandler.Me)
			protected.Patch("/me/profile", registerHandler.UpdateProfile)
			protected.Get("/transport/search", cargoHandler.SearchAvailableTransport)

			// Прямое предложение перевозчику из поиска транспорта: торг по
			// цене, при согласии — раскрытие контактов и общий чат.
			protected.Post("/transport/{vehicleId}/proposals", cargoHandler.SendTransportProposal)
			protected.Get("/transport-proposals/mine", cargoHandler.ListMyTransportProposals)
			protected.Get("/transport-proposals/incoming", cargoHandler.ListIncomingTransportProposals)
			protected.Post("/transport-proposals/{id}/quote", cargoHandler.QuoteTransportProposal)
			protected.Post("/transport-proposals/{id}/counter", cargoHandler.CounterTransportProposal)
			protected.Post("/transport-proposals/{id}/final", cargoHandler.FinalTransportProposal)
			protected.Post("/transport-proposals/{id}/accept", cargoHandler.AcceptTransportProposal)
			protected.Post("/transport-proposals/{id}/reject", cargoHandler.RejectTransportProposal)

			// Мои инструменты (без роли): участник сам включает/выключает.
			protected.Get("/my/tools", cargoHandler.GetMyTools)
			protected.Put("/my/tools", cargoHandler.SetMyTools)

			// Scaffolding for the tool-based access principle — see
			// handlers.ToolAccessCheck for why this route exists.
			protected.With(appmiddleware.RequireTool(toolRepo, "key")).
				Get("/tools/{key}/access-check", handlers.ToolAccessCheck)

			protected.Post("/cargo", cargoHandler.CreateCargoRequest)
			protected.Get("/cargo/mine", cargoHandler.ListMyCargoRequests)
			protected.Put("/cargo/{id}", cargoHandler.UpdateCargoRequest)
			protected.Delete("/cargo/{id}", cargoHandler.CancelCargoRequest)
			protected.Get("/cargo/available", cargoHandler.ListAvailableCargoRequests)
			protected.Get("/offers/mine", cargoHandler.ListMyCargoCompetitionResponses)
			protected.Put("/offers/{id}", cargoHandler.UpdateMyOffer)
			protected.Delete("/offers/{id}", cargoHandler.WithdrawMyOffer)
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
			protected.Patch("/fleet/{id}", cargoHandler.UpdateMyVehicleDetails)
			protected.Patch("/fleet/{id}/name", cargoHandler.UpdateMyVehicleName)
			protected.Patch("/fleet/{id}/location", cargoHandler.UpdateMyVehicleLocation)
			protected.Patch("/fleet/{id}/registration", cargoHandler.UpdateMyVehicleRegistration)
			protected.Post("/fleet/{id}/documents", cargoHandler.UploadMyVehicleDocument)
			protected.Post("/fleet/{id}/destinations", cargoHandler.AddMyVehicleDestination)
			protected.Delete("/fleet/{id}/destinations/{did}", cargoHandler.DeleteMyVehicleDestination)
			protected.Post("/fleet/{id}/trips", cargoHandler.AddMyVehicleTrip)
			protected.Put("/fleet/{id}/trips/{tid}", cargoHandler.UpdateMyVehicleTrip)
			protected.Delete("/fleet/{id}/trips/{tid}", cargoHandler.DeleteMyVehicleTrip)
			protected.Delete("/fleet/{id}", cargoHandler.DeleteMyVehicle)

			// Пороги отправки склада (manage_warehouse_slots, ТЗ §5.2).
			protected.Get("/dispatch-thresholds", cargoHandler.ListMyDispatchThresholds)
			protected.Put("/routes/{id}/dispatch-threshold", cargoHandler.SetRouteDispatchThreshold)
			protected.Delete("/routes/{id}/dispatch-threshold", cargoHandler.DeleteRouteDispatchThreshold)

			protected.Get("/warehouses/search", cargoHandler.SearchWarehouses)

			// Склады как ставщики цены на груз (Фаза 2): владелец склада видит
			// подходящий груз и предлагает цену; клиент выбирает склад → чат.
			protected.Get("/warehouse/available-cargo", cargoHandler.ListCargoForMyWarehouses)
			protected.Post("/cargo/{id}/warehouse-offers", cargoHandler.SubmitWarehouseOffer)
			protected.Get("/cargo/{id}/warehouse-offers", cargoHandler.ListWarehouseOffersForCargo)
			protected.Post("/cargo/{id}/warehouse-offers/{offerId}/select", cargoHandler.SelectWarehouseOffer)

			// Фаза 3: склады ставят цену на консолидированную заявку.
			protected.Get("/warehouse/available-consolidated", cargoHandler.ListConsolidatedForMyWarehouses)
			protected.Post("/consolidated/{id}/warehouse-offers", cargoHandler.SubmitWarehouseOfferForConsolidated)
			protected.Get("/consolidated/{id}/warehouse-offers", cargoHandler.ListWarehouseOffersForConsolidated)
			protected.Post("/consolidated/{id}/warehouse-offers/{offerId}/select", cargoHandler.SelectWarehouseOfferForConsolidated)

			// Поздний до-запрос: присоединить свой груз к консолидации (Фаза 3b).
			protected.Get("/cargo/{id}/matching-consolidations", cargoHandler.ListMatchingConsolidationsForCargo)
			protected.Post("/consolidated/{id}/join", cargoHandler.JoinConsolidation)

			protected.Get("/warehouses/mine", cargoHandler.ListMyWarehouses)
			protected.Post("/warehouses", cargoHandler.CreateMyWarehouse)
			protected.Put("/warehouses/{id}", cargoHandler.UpdateMyWarehouse)
			protected.Delete("/warehouses/{id}", cargoHandler.DeleteMyWarehouse)

			// Конкурс водителей (ТЗ §11.4): склад объявляет и выбирает,
			// водители ставят цены.
			protected.Post("/driver-competitions", cargoHandler.CreateDriverCompetition)
			protected.Get("/driver-competitions/mine", cargoHandler.ListMyDriverCompetitions)
			protected.Get("/driver-competitions/open", cargoHandler.ListOpenDriverCompetitions)
			protected.Get("/driver-competitions/responses", cargoHandler.ListMyDriverCompetitionResponses)
			protected.Put("/driver-competitions/{id}", cargoHandler.UpdateDriverCompetition)
			protected.Delete("/driver-competitions/{id}", cargoHandler.CancelDriverCompetition)
			protected.Post("/driver-competitions/{id}/bids", cargoHandler.CreateDriverBid)
			protected.Put("/driver-bids/{id}", cargoHandler.UpdateMyDriverBid)
			protected.Delete("/driver-bids/{id}", cargoHandler.WithdrawMyDriverBid)
			protected.Post("/driver-competitions/{id}/bids/{bid}/select", cargoHandler.SelectDriverBid)

			// Конкурс таможенных представителей (manage_customs_docs, ТЗ §10.2).
			protected.Get("/customs/competitions", cargoHandler.ListCustomsCompetitions)
			protected.Get("/customs/competitions/responses", cargoHandler.ListMyCustomsCompetitionResponses)
			protected.Post("/consolidated/{id}/customs-offers", cargoHandler.CreateCustomsOffer)
			protected.Put("/customs-offers/{id}", cargoHandler.UpdateMyCustomsOffer)
			protected.Delete("/customs-offers/{id}", cargoHandler.WithdrawMyCustomsOffer)
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
			admin.Post("/logout", adminHandler.Logout)

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
				protected.Get("/vehicle-verifications", adminHandler.VehicleVerificationQueue)
				protected.Get("/vehicle-verifications/{id}", adminHandler.VehicleVerificationDetail)
				protected.Post("/vehicle-verifications/{id}/approve", adminHandler.ApproveVehicleVerification)
				protected.Post("/vehicle-verifications/{id}/reject", adminHandler.RejectVehicleVerification)

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

	// Background sweep: resolve consolidation suggestions whose response window
	// (3h) has elapsed, so a group forms even if some members never answered.
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Isolate each tick: a panic here (e.g. bad data) must not take
				// the whole process down — HTTP Recoverer doesn't cover goroutines.
				func() {
					defer func() {
						if rec := recover(); rec != nil {
							log.Printf("resolve expired consolidations panicked: %v", rec)
						}
					}()
					if err := cargoSvc.ResolveExpiredConsolidations(context.Background()); err != nil {
						log.Printf("resolve expired consolidations: %v", err)
					}
				}()
			}
		}
	}()

	// Background refresh of NBK exchange rates: once on startup, then every 6h
	// (NBK publishes daily; the extra ticks retry if a fetch failed). Fetch
	// failures keep the last good rates. Best-effort, isolated with recover.
	go func() {
		refresh := func() {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("currency rates refresh panicked: %v", rec)
				}
			}()
			fetchCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := ratesSvc.Refresh(fetchCtx); err != nil {
				log.Printf("currency rates refresh: %v", err)
			}
		}
		refresh()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				refresh()
			}
		}
	}()

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
