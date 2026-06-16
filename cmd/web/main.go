package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"catalogo/internal/config"
	"catalogo/internal/database"
	"catalogo/internal/handlers"
	"catalogo/internal/middleware"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

func main() {
	// Carrega configurações
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Erro ao carregar configuração: %v", err)
	}

	// Conecta ao banco de dados
	db, err := database.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Erro ao conectar ao banco: %v", err)
	}
	defer db.Close()

	// Executa migrações
	if err := db.RunMigrations(); err != nil {
		log.Fatalf("Erro ao executar migrações: %v", err)
	}
	log.Println("✅ Migrações executadas com sucesso")

	// Inicializa handlers
	h := handlers.NewHandlers(db, cfg.DevMode)

	// Cria roteador
	r := chi.NewRouter()

	// Middlewares globais
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Compress(5))

	// Arquivos estáticos
	fileServer := http.FileServer(http.Dir("public"))
	r.Handle("/static/*", http.StripPrefix("/static/", fileServer))

	// ==================== ROTAS PÚBLICAS ====================

	// Landing page
	r.Get("/", h.HandleHome)

	// Auth (login/register - sem autenticação)
	r.Get("/admin/login", h.HandleLoginPage)
	r.Post("/admin/login", h.HandleLoginPost)
	r.Get("/admin/register", h.HandleRegisterPage)
	r.Post("/admin/register", h.HandleRegisterPost)
	r.Get("/admin/logout", h.HandleLogout)

	// ==================== ROTAS ADMIN (autenticadas) ====================

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireAuth(db))

		// Dashboard
		r.Get("/admin", h.HandleDashboard)

		// Produtos
		r.Get("/admin/produtos", h.HandleProducts)
		r.Post("/admin/produtos", h.HandleCreateProduct)
		r.Post("/admin/produtos/{id}", h.HandleUpdateProduct)
		r.Delete("/admin/produtos/{id}", h.HandleDeleteProduct)
		r.Patch("/admin/produtos/{id}/toggle", h.HandleToggleProduct)

		// Categorias
		r.Get("/admin/categorias", h.HandleCategories)
		r.Post("/admin/categorias", h.HandleCreateCategory)
		r.Delete("/admin/categorias/{id}", h.HandleDeleteCategory)

		// Configurações da loja
		r.Get("/admin/config", h.HandleShopConfig)
		r.Post("/admin/config", h.HandleShopConfigPost)

		// Pedidos do lojista
		r.Get("/admin/pedidos", h.HandleOrders)
		r.Post("/admin/pedidos/{id}/status", h.HandleOrderStatusPost)

		// Cupons de desconto
		r.Get("/admin/cupons", h.HandleCoupons)
		r.Post("/admin/cupons", h.HandleCreateCoupon)
		r.Delete("/admin/cupons/{id}", h.HandleDeleteCoupon)

		// Assinatura e Faturamento
		r.Get("/admin/plano", h.HandleShopBilling)
		r.Post("/admin/plano/upgrade", h.HandleUpgradeSimulatorPost)

		// Relatórios de Vendas
		r.Get("/admin/relatorios", h.HandleReportsPage)
		r.Get("/admin/relatorios/impressao", h.HandlePrintReports)

		// Panfleto / Display de Mesa QR Code
		r.Get("/admin/flyer", h.HandleQRFlyer)

		// Banners Promocionais
		r.Get("/admin/banners", h.HandleManageBanners)
		r.Post("/admin/banners", h.HandleCreateBanner)
		r.Delete("/admin/banners/{id}", h.HandleDeleteBanner)
	})

	// ==================== ROTAS MASTER ADMIN (SaaS Subsystem) ====================
	r.Get("/master/login", h.HandleMasterLoginPage)
	r.Post("/master/login", h.HandleMasterLoginPost)
	r.Get("/master/logout", h.HandleMasterLogout)

	r.Group(func(r chi.Router) {
		r.Use(middleware.RequireMasterAuth(db))

		r.Get("/master", h.HandleMasterDashboard)
		r.Post("/master/shops/{id}/toggle", h.HandleMasterToggleShop)
		r.Post("/master/shops/{id}/plan", h.HandleMasterChangePlan)
		r.Post("/master/configs", h.HandleMasterUpdateConfigs)
	})

	// ==================== ROTAS CATÁLOGO PÚBLICO ====================
	// Deve ficar por último para não conflitar com as rotas acima

	r.Get("/{slug}", h.HandleCatalog)
	r.Get("/{slug}/produtos", h.HandleProductsByCategory)
	r.Post("/{slug}/checkout", h.HandleCheckout)
	r.Get("/{slug}/coupon/{code}", h.HandleValidateCoupon)

	// ==================== SERVIDOR ====================

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("🔄 Encerrando servidor...")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			log.Fatalf("Erro ao encerrar servidor: %v", err)
		}
	}()

	log.Printf("🚀 Servidor rodando em http://localhost%s", addr)
	log.Printf("📋 Catálogo público: http://localhost%s/{slug-da-loja}", addr)
	log.Printf("🔧 Painel admin: http://localhost%s/admin", addr)

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Erro ao iniciar servidor: %v", err)
	}

	log.Println("✅ Servidor encerrado")
}
