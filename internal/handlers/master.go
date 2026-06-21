package handlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"catalogo/internal/database"
	"catalogo/internal/middleware"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// HandleMasterLoginPage renderiza a página de login exclusiva do Master Admin
func (h *Handlers) HandleMasterLoginPage(w http.ResponseWriter, r *http.Request) {
	errParam := r.URL.Query().Get("error")
	data := map[string]interface{}{
		"Error": errParam,
	}
	
	if err := h.Tmpl.RenderPage(w, "master/login.html", data); err != nil {
		log.Printf("[MASTER] Erro ao renderizar login: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleMasterLoginPost valida as credenciais exclusivas do Master Admin e cria sessão separada
func (h *Handlers) HandleMasterLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/master/login?error=Dados inválidos", http.StatusSeeOther)
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	// 1. Busca usuário e verifica se possui flag super admin
	user, err := h.DB.GetUserByEmail(r.Context(), email)
	if err != nil || !user.IsSuperAdmin {
		http.Redirect(w, r, "/master/login?error=Credenciais inválidas ou acesso não autorizado", http.StatusSeeOther)
		return
	}

	// 2. Compara senhas com bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		http.Redirect(w, r, "/master/login?error=Credenciais inválidas ou acesso não autorizado", http.StatusSeeOther)
		return
	}

	// 3. Cria sessão no banco de dados
	session, err := h.DB.CreateSession(r.Context(), user.ID)
	if err != nil {
		log.Printf("[MASTER] Erro ao criar sessão: %v", err)
		http.Redirect(w, r, "/master/login?error=Erro interno no servidor", http.StatusSeeOther)
		return
	}

	// 4. Grava cookie exclusivo do SaaS ("master_session_id")
	http.SetCookie(w, &http.Cookie{
		Name:     "master_session_id",
		Value:    session.ID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/master", http.StatusSeeOther)
}

// HandleMasterLogout realiza o log-out exclusivo do Master Admin
func (h *Handlers) HandleMasterLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("master_session_id")
	if err == nil {
		_ = h.DB.DeleteSession(r.Context(), cookie.Value)
	}

	// Limpa o cookie SaaS
	http.SetCookie(w, &http.Cookie{
		Name:     "master_session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/master/login", http.StatusSeeOther)
}

// HandleMasterDashboard renderiza o painel de controle mestre do SaaS (Visão Geral - Métricas)
func (h *Handlers) HandleMasterDashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)

	// 1. Busca estatísticas globais
	metrics, err := h.DB.GetPlatformMetrics(r.Context())
	if err != nil {
		log.Printf("[MASTER] Erro ao obter métricas da plataforma: %v", err)
		metrics = map[string]interface{}{
			"total_users":    0,
			"total_shops":    0,
			"total_orders":   0,
			"global_revenue": 0.0,
			"saas_revenue":   0.0,
		}
	}

	data := map[string]interface{}{
		"User":      user,
		"Metrics":   metrics,
		"ActiveTab": "dashboard",
	}

	// Renderiza usando o layout exclusivo do super admin ("super")
	if err := h.Tmpl.Render(w, "super", "master/dashboard.html", data); err != nil {
		log.Printf("[MASTER] Erro ao renderizar master dashboard: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleMasterSettings renderiza a tela de configurações globais e planos do SaaS
func (h *Handlers) HandleMasterSettings(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)

	// 1. Busca configurações globais
	configs, err := h.DB.GetPlatformConfigs(r.Context())
	if err != nil {
		log.Printf("[MASTER] Erro ao carregar configurações: %v", err)
		configs = map[string]string{
			"platform_name":    "Catálogo Digital SaaS",
			"maintenance_mode": "false",
			"support_whatsapp": "5511999999999",
		}
	}

	// 2. Busca planos cadastrados
	rows, err := h.DB.Pool.Query(r.Context(), "SELECT id, name, price, max_products, max_categories FROM plans ORDER BY id ASC")
	var plans []database.Plan
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p database.Plan
			if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.MaxProducts, &p.MaxCategories); err == nil {
				plans = append(plans, p)
			}
		}
	} else {
		log.Printf("[MASTER] Erro ao carregar planos: %v", err)
	}

	// 3. Busca logs de auditoria
	auditLogs, err := h.DB.ListPlatformAuditLogs(r.Context())
	if err != nil {
		log.Printf("[MASTER] Erro ao carregar logs de auditoria: %v", err)
		auditLogs = []database.PlatformAuditLog{}
	}

	data := map[string]interface{}{
		"User":      user,
		"Configs":   configs,
		"Plans":     plans,
		"AuditLogs": auditLogs,
		"ActiveTab": "settings",
	}

	// Renderiza usando o layout exclusivo do super admin ("super")
	if err := h.Tmpl.Render(w, "super", "master/settings.html", data); err != nil {
		log.Printf("[MASTER] Erro ao renderizar master settings: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleMasterShops lista todas as lojas cadastradas para o super admin com suporte a busca e paginação
func (h *Handlers) HandleMasterShops(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	search := r.URL.Query().Get("search")

	// Busca lista de lojas de forma paginada (10 por página)
	page := 1
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	limit := 10
	offset := (page - 1) * limit

	shops, totalShops, err := h.DB.ListShopsWithOwnersPaginated(r.Context(), limit, offset, search)
	if err != nil {
		log.Printf("[MASTER] Erro ao listar lojas: %v", err)
		shops = []database.ShopWithOwner{}
	}

	totalPages := (totalShops + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	data := map[string]interface{}{
		"User":           user,
		"Shops":          shops,
		"ActiveTab":      "shops",
		"CurrentPage":    page,
		"TotalPages":     totalPages,
		"ShowPagination": totalPages > 1,
		"HasPrevPage":    page > 1,
		"HasNextPage":    page < totalPages,
		"PrevPage":       page - 1,
		"NextPage":       page + 1,
		"SearchTerm":     search,
	}

	// Renderiza usando o layout exclusivo do super admin ("super")
	if err := h.Tmpl.Render(w, "super", "master/shops.html", data); err != nil {
		log.Printf("[MASTER] Erro ao renderizar master lojas: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleMasterBilling lista faturas globais com filtros, métricas e gráficos para o super admin
func (h *Handlers) HandleMasterBilling(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)

	// Captura filtros da URL
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")
	yearStr := r.URL.Query().Get("year")

	year := 0
	if yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil {
			year = y
		}
	}

	// Se for o primeiro carregamento (URL limpa), define últimos 30 dias por padrão
	isFiltered := r.URL.Query().Has("start_date") || r.URL.Query().Has("end_date") || r.URL.Query().Has("year")
	if !isFiltered {
		endDate = time.Now().Format("2006-01-02")
		startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}

	// Busca paginação
	page := 1
	if pStr := r.URL.Query().Get("page"); pStr != "" {
		if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
			page = p
		}
	}
	limit := 10
	offset := (page - 1) * limit

	// 1. Busca faturas filtradas e paginadas
	charges, totalCharges, err := h.DB.ListGlobalChargesFiltered(r.Context(), limit, offset, startDate, endDate, year)
	if err != nil {
		log.Printf("[MASTER] Erro ao carregar faturas filtradas: %v", err)
		charges = []database.PaymentChargeWithDetails{}
	}

	totalPages := (totalCharges + limit - 1) / limit
	if totalPages < 1 {
		totalPages = 1
	}

	// 2. Busca todas as faturas do filtro para calcular totais/estatísticas do período
	allFiltered, err := h.DB.ListGlobalChargesFilteredAll(r.Context(), startDate, endDate, year)
	var totalRevenue float64
	var paidCount int
	var pendingCount int
	if err == nil {
		for _, c := range allFiltered {
			if c.Status == "RECEIVED" || c.Status == "CONFIRMED" {
				totalRevenue += c.Amount
				paidCount++
			} else if c.Status == "PENDING" {
				pendingCount++
			}
		}
	} else {
		log.Printf("[MASTER] Erro ao carregar totalizador de faturas: %v", err)
	}

	// 3. Busca pontos do gráfico de faturamento
	chartPoints, err := h.DB.GetSaaSRevenueChartData(r.Context(), startDate, endDate, year)
	if err != nil {
		log.Printf("[MASTER] Erro ao obter dados do gráfico de faturamento: %v", err)
		chartPoints = []database.SaaSRevenuePoint{}
	}

	// Converte os pontos para JSON
	chartJSON := "[]"
	if bytes, err := json.Marshal(chartPoints); err == nil {
		chartJSON = string(bytes)
	}

	// 4. Busca os anos distintos de cobranças para popular o dropdown
	years, err := h.DB.GetDistinctBillingYears(r.Context())
	if err != nil {
		log.Printf("[MASTER] Erro ao buscar anos distintos de faturamento: %v", err)
		years = []int{time.Now().Year()}
	}

	data := map[string]interface{}{
		"User":           user,
		"Charges":        charges,
		"CurrentPage":    page,
		"TotalPages":     totalPages,
		"ShowPagination": totalPages > 1,
		"HasPrevPage":    page > 1,
		"HasNextPage":    page < totalPages,
		"PrevPage":       page - 1,
		"NextPage":       page + 1,
		"ActiveTab":      "billing",
		
		// Filtros e Estatísticas do Período
		"StartDate":      startDate,
		"EndDate":        endDate,
		"SelectedYear":   year,
		"Years":          years,
		
		"TotalRevenue":   totalRevenue,
		"TotalCharges":   len(allFiltered),
		"PaidCount":      paidCount,
		"PendingCount":   pendingCount,
		
		// JSON para o gráfico analítico
		"ChartDataJSON":  chartJSON,
	}

	// Renderiza usando o layout exclusivo do super admin ("super")
	if err := h.Tmpl.Render(w, "super", "master/billing.html", data); err != nil {
		log.Printf("[MASTER] Erro ao renderizar master faturamento: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleMasterToggleShop suspende ou reativa uma loja e grava no log de auditoria
func (h *Handlers) HandleMasterToggleShop(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	shopID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	// Busca informações da loja antes de alterar
	var shopName string
	err = h.DB.Pool.QueryRow(r.Context(), "SELECT name FROM shops WHERE id = $1", shopID).Scan(&shopName)
	if err != nil {
		log.Printf("[MASTER] Erro ao buscar nome da loja %d: %v", shopID, err)
		shopName = fmt.Sprintf("ID #%d", shopID)
	}

	// Alterna status no banco
	active, err := h.DB.ToggleShopActive(r.Context(), shopID)
	if err != nil {
		log.Printf("[MASTER] Erro ao alterar status da loja %d: %v", shopID, err)
		http.Error(w, "Erro ao atualizar status", http.StatusInternalServerError)
		return
	}

	// Registra a ação no log de auditoria do Master Admin
	var logAction string
	var logDetails string
	if active {
		logAction = "SHOP_ACTIVATE"
		logDetails = fmt.Sprintf("Loja '%s' (ID %d) foi reativada e restabelecida com sucesso.", shopName, shopID)
	} else {
		logAction = "SHOP_SUSPEND"
		logDetails = fmt.Sprintf("Loja '%s' (ID %d) foi suspensa por descumprimento de termos ou inadimplência.", shopName, shopID)
	}

	if err := h.DB.CreatePlatformAuditLog(r.Context(), logAction, logDetails); err != nil {
		log.Printf("[MASTER] Falha ao gravar log de auditoria: %v", err)
	}

	// Retorna o HTML parcial para atualizar a linha da tabela via HTMX
	w.Header().Set("Content-Type", "text/html")
	if active {
		w.Write([]byte(`
			<div class="flex items-center justify-end gap-2">
				<span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-bold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
					<span class="w-1.5 h-1.5 rounded-full bg-emerald-400 mr-1.5 animate-pulse"></span>
					Ativa
				</span>
				<button hx-post="/master/shops/` + idStr + `/toggle" 
				        hx-swap="outerHTML" 
				        hx-target="closest div" 
				        class="px-2.5 py-1 rounded-lg bg-red-500/10 hover:bg-red-500/20 text-red-400 border border-red-500/20 text-xs font-bold transition-all active:scale-[0.98]">
					Suspender
				</button>
			</div>
		`))
	} else {
		w.Write([]byte(`
			<div class="flex items-center justify-end gap-2">
				<span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-bold bg-red-500/10 text-red-400 border border-red-500/20">
					Suspensa
				</span>
				<button hx-post="/master/shops/` + idStr + `/toggle" 
				        hx-swap="outerHTML" 
				        hx-target="closest div" 
				        class="px-2.5 py-1 rounded-lg bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-400 border border-emerald-500/20 text-xs font-bold transition-all active:scale-[0.98]">
					Reativar
				</button>
			</div>
		`))
	}
}

// HandleMasterUpdateConfigs salva configurações globais do SaaS e grava audit log
func (h *Handlers) HandleMasterUpdateConfigs(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	platformName := r.FormValue("platform_name")
	subFee := r.FormValue("global_subscription_fee")
	whatsapp := r.FormValue("support_whatsapp")
	mMode := r.FormValue("maintenance_mode")

	if mMode == "" {
		mMode = "false"
	}

	// Grava configurações no banco
	configs := map[string]string{
		"platform_name":           platformName,
		"global_subscription_fee": subFee,
		"support_whatsapp":        whatsapp,
		"maintenance_mode":        mMode,
	}

	for k, v := range configs {
		if err := h.DB.UpdatePlatformConfig(r.Context(), k, v); err != nil {
			log.Printf("[MASTER] Erro ao salvar config %s: %v", k, err)
			http.Error(w, "Erro ao salvar configurações", http.StatusInternalServerError)
			return
		}
	}

	// Grava log de auditoria
	details := fmt.Sprintf("Configurações atualizadas: Nome=%s, Mensalidade=R$%s, Whats=%s, Manutenção=%s", 
		platformName, subFee, whatsapp, mMode)
	if err := h.DB.CreatePlatformAuditLog(r.Context(), "CONFIG_UPDATE", details); err != nil {
		log.Printf("[MASTER] Falha ao gravar log de auditoria: %v", err)
	}

	// Trigger total refresh para atualizar os valores salvos e o log de auditoria na tela
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// HandleMasterChangePlan força a alteração manual do plano e validade de uma loja
func (h *Handlers) HandleMasterChangePlan(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	shopID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	planIDStr := r.FormValue("plan_id")
	planID, err := strconv.Atoi(planIDStr)
	if err != nil {
		http.Error(w, "Plano inválido", http.StatusBadRequest)
		return
	}

	// 1. Busca detalhes do plano
	plan, err := h.DB.GetPlanByID(r.Context(), planID)
	if err != nil {
		http.Error(w, "Plano não encontrado", http.StatusNotFound)
		return
	}

	// 2. Busca informações da loja
	var shopName string
	err = h.DB.Pool.QueryRow(r.Context(), "SELECT name FROM shops WHERE id = $1", shopID).Scan(&shopName)
	if err != nil {
		log.Printf("[MASTER] Erro ao buscar nome da loja %d: %v", shopID, err)
		shopName = fmt.Sprintf("ID #%d", shopID)
	}

	// 3. Define validade (30 dias para pagos, indefinido para Bronze/Grátis)
	days := 30
	if planID == 1 {
		days = 0
	}

	// 4. Aplica alteração no banco
	if err := h.DB.UpgradeShopPlan(r.Context(), shopID, planID, days); err != nil {
		log.Printf("[MASTER] Erro ao forçar alteração de plano da loja %d: %v", shopID, err)
		http.Error(w, "Erro ao processar alteração", http.StatusInternalServerError)
		return
	}

	// 5. Registra ação no Audit Log SaaS
	details := fmt.Sprintf("Alteração manual de plano executada pelo Admin Mestre. Loja: %s (ID %d). Novo Plano: %s.", 
		shopName, shopID, plan.Name)
	if err := h.DB.CreatePlatformAuditLog(r.Context(), "PLAN_CHANGE_FORCED", details); err != nil {
		log.Printf("[MASTER] Erro ao criar log de auditoria SaaS: %v", err)
	}

	// Força recarregamento total da página master para atualizar os dados
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// HandleMasterConfirmCharge aprova manualmente um pagamento e ativa o plano correspondente
func (h *Handlers) HandleMasterConfirmCharge(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	chargeID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID de cobrança inválido", http.StatusBadRequest)
		return
	}

	// 1. Busca detalhes da cobrança no banco
	charge, err := h.DB.GetChargeByID(r.Context(), chargeID)
	if err != nil {
		log.Printf("[MASTER] Erro ao buscar cobrança %d: %v", chargeID, err)
		http.Error(w, "Cobrança não encontrada", http.StatusNotFound)
		return
	}

	if charge.Status != "PENDING" {
		http.Error(w, "Esta cobrança já foi processada anteriormente", http.StatusBadRequest)
		return
	}

	// 2. Busca informações do plano
	plan, err := h.DB.GetPlanByID(r.Context(), charge.PlanID)
	if err != nil {
		log.Printf("[MASTER] Erro ao buscar plano %d: %v", charge.PlanID, err)
		http.Error(w, "Plano associado não encontrado", http.StatusNotFound)
		return
	}

	// 3. Busca informações da loja
	var shopName string
	err = h.DB.Pool.QueryRow(r.Context(), "SELECT name FROM shops WHERE id = $1", charge.ShopID).Scan(&shopName)
	if err != nil {
		log.Printf("[MASTER] Erro ao buscar nome da loja %d: %v", charge.ShopID, err)
		shopName = fmt.Sprintf("ID #%d", charge.ShopID)
	}

	// 4. Ativa o plano da loja por 30 dias no banco
	if err := h.DB.UpgradeShopPlan(r.Context(), charge.ShopID, charge.PlanID, 30); err != nil {
		log.Printf("[MASTER] Erro ao atualizar plano para shop %d: %v", charge.ShopID, err)
		http.Error(w, "Erro ao atualizar plano no banco de dados", http.StatusInternalServerError)
		return
	}

	// 5. Atualiza status da cobrança no banco para RECEIVED
	if err := h.DB.UpdateChargeStatus(r.Context(), charge.AsaasPaymentID, "RECEIVED"); err != nil {
		log.Printf("[MASTER] Erro ao atualizar status da cobrança %s: %v", charge.AsaasPaymentID, err)
	}

	// 6. Grava log de auditoria
	details := fmt.Sprintf("Confirmação de faturamento manual realizada pelo Admin Mestre. Loja: %s (ID %d). Plano: %s. Asaas ID: %s. Valor: R$ %.2f.", 
		shopName, charge.ShopID, plan.Name, charge.AsaasPaymentID, charge.Amount)
	if err := h.DB.CreatePlatformAuditLog(r.Context(), "PAYMENT_CONFIRM_MANUAL", details); err != nil {
		log.Printf("[MASTER] Erro ao criar log de auditoria SaaS: %v", err)
	}

	log.Printf("[MASTER] Cobrança %d confirmada manualmente (Loja ID %d, Plano: %s)", chargeID, charge.ShopID, plan.Name)

	// Força recarregamento da página para atualizar dados de vencimento e auditoria
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// HandleMasterUpdatePlan atualiza os limites e preço de um plano
func (h *Handlers) HandleMasterUpdatePlan(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	planID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID de plano inválido", http.StatusBadRequest)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Formulário inválido", http.StatusBadRequest)
		return
	}

	priceStr := r.FormValue("price")
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		http.Error(w, "Preço inválido", http.StatusBadRequest)
		return
	}

	maxProductsStr := r.FormValue("max_products")
	maxProducts, err := strconv.Atoi(maxProductsStr)
	if err != nil {
		http.Error(w, "Limite de produtos inválido", http.StatusBadRequest)
		return
	}

	maxCategoriesStr := r.FormValue("max_categories")
	maxCategories, err := strconv.Atoi(maxCategoriesStr)
	if err != nil {
		http.Error(w, "Limite de categorias inválido", http.StatusBadRequest)
		return
	}

	// 1. Busca nome do plano antes de atualizar
	var planName string
	err = h.DB.Pool.QueryRow(r.Context(), "SELECT name FROM plans WHERE id = $1", planID).Scan(&planName)
	if err != nil {
		log.Printf("[MASTER] Erro ao buscar nome do plano %d: %v", planID, err)
		planName = fmt.Sprintf("ID #%d", planID)
	}

	// 2. Atualiza no banco
	if err := h.DB.UpdatePlan(r.Context(), planID, price, maxProducts, maxCategories); err != nil {
		log.Printf("[MASTER] Erro ao atualizar plano %d: %v", planID, err)
		http.Error(w, "Erro ao atualizar plano", http.StatusInternalServerError)
		return
	}

	// 3. Registra log de auditoria
	details := fmt.Sprintf("Plano '%s' (ID %d) atualizado: Preço=R$ %.2f, MaxProdutos=%d, MaxCategorias=%d",
		planName, planID, price, maxProducts, maxCategories)
	if err := h.DB.CreatePlatformAuditLog(r.Context(), "PLAN_CONFIG_UPDATE", details); err != nil {
		log.Printf("[MASTER] Erro ao criar log de auditoria: %v", err)
	}

	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}

// HandleMasterExportBillingCSV exporta todo o histórico de faturamento filtrado em CSV
func (h *Handlers) HandleMasterExportBillingCSV(w http.ResponseWriter, r *http.Request) {
	// Captura os mesmos filtros da tela
	startDate := r.URL.Query().Get("start_date")
	endDate := r.URL.Query().Get("end_date")
	yearStr := r.URL.Query().Get("year")

	year := 0
	if yearStr != "" {
		if y, err := strconv.Atoi(yearStr); err == nil {
			year = y
		}
	}

	// Se for URL limpa, define últimos 30 dias por padrão
	isFiltered := r.URL.Query().Has("start_date") || r.URL.Query().Has("end_date") || r.URL.Query().Has("year")
	if !isFiltered {
		endDate = time.Now().Format("2006-01-02")
		startDate = time.Now().AddDate(0, 0, -30).Format("2006-01-02")
	}

	charges, err := h.DB.ListGlobalChargesFilteredAll(r.Context(), startDate, endDate, year)
	if err != nil {
		log.Printf("[MASTER] Erro ao carregar cobranças para CSV: %v", err)
		http.Error(w, "Erro ao carregar dados", http.StatusInternalServerError)
		return
	}

	// Define os headers de resposta para download do arquivo CSV
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=relatorio_fiscal_faturamento_%s.csv", time.Now().Format("2006-01-02_150405")))

	// Escreve o BOM UTF-8 para garantir que o Excel abra os acentos corretamente
	w.Write([]byte{0xEF, 0xBB, 0xBF})

	writer := csv.NewWriter(w)
	writer.Comma = ';' // Delimitador padrão do Excel em português

	// Cabeçalhos das colunas
	headers := []string{
		"ID Interno",
		"ID Transacao Asaas",
		"Loja",
		"Proprietario",
		"Email Proprietario",
		"WhatsApp",
		"Plano Contratado",
		"Metodo",
		"Valor (R$)",
		"Status",
		"Data Criacao",
		"Vencimento",
	}
	if err := writer.Write(headers); err != nil {
		log.Printf("[MASTER] Erro ao escrever cabeçalho do CSV: %v", err)
		return
	}

	for _, c := range charges {
		// Busca a loja e o proprietário para trazer dados completos e auditáveis
		ownerName := "Desconhecido"
		ownerEmail := "Desconhecido"
		whatsapp := ""
		shop, sErr := h.DB.GetShopByID(r.Context(), c.ShopID)
		if sErr == nil {
			whatsapp = shop.WhatsappNumber
			user, uErr := h.DB.GetUserByID(r.Context(), shop.UserID)
			if uErr == nil {
				ownerName = user.Name
				ownerEmail = user.Email
			}
		}

		expiresStr := "Sem Vencimento"
		if c.ExpiresAt != nil {
			expiresStr = c.ExpiresAt.Format("02/01/2006 15:04")
		}

		billingType := c.BillingType
		if billingType == "CREDIT_CARD" {
			billingType = "Cartão de Crédito"
		}

		amountStr := fmt.Sprintf("%.2f", c.Amount)
		amountStr = strings.ReplaceAll(amountStr, ".", ",") // padrão numérico local

		row := []string{
			strconv.Itoa(c.ID),
			c.AsaasPaymentID,
			c.ShopName,
			ownerName,
			ownerEmail,
			whatsapp,
			c.PlanName,
			billingType,
			amountStr,
			c.Status,
			c.CreatedAt.Format("02/01/2006 15:04"),
			expiresStr,
		}

		if err := writer.Write(row); err != nil {
			log.Printf("[MASTER] Erro ao escrever linha do CSV: %v", err)
			return
		}
	}

	writer.Flush()
}

// HandleMasterPrintInvoiceReceipt renderiza o recibo/comprovante individual da transação
func (h *Handlers) HandleMasterPrintInvoiceReceipt(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	chargeID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID de fatura inválido", http.StatusBadRequest)
		return
	}

	// 1. Busca a cobrança
	charge, err := h.DB.GetChargeByID(r.Context(), chargeID)
	if err != nil {
		http.Error(w, "Fatura não encontrada", http.StatusNotFound)
		return
	}

	// 2. Busca a loja correspondente
	shop, err := h.DB.GetShopByID(r.Context(), charge.ShopID)
	if err != nil {
		http.Error(w, "Loja associada não encontrada", http.StatusNotFound)
		return
	}

	// 3. Busca o plano faturado
	plan, err := h.DB.GetPlanByID(r.Context(), charge.PlanID)
	if err != nil {
		plan = &database.Plan{Name: "Bronze (Grátis)", Price: 0.0}
	}

	// 4. Busca os dados do proprietário (User)
	ownerName := "Desconhecido"
	ownerEmail := "Desconhecido"
	user, err := h.DB.GetUserByID(r.Context(), shop.UserID)
	if err == nil {
		ownerName = user.Name
		ownerEmail = user.Email
	}

	data := map[string]interface{}{
		"Charge":     charge,
		"Shop":       shop,
		"Plan":       plan,
		"OwnerName":  ownerName,
		"OwnerEmail": ownerEmail,
		"Now":        time.Now(),
	}

	// Renderiza a página standalone de impressão (sem layouts)
	if err := h.Tmpl.RenderPage(w, "master/print_invoice.html", data); err != nil {
		log.Printf("[MASTER] Erro ao renderizar recibo de faturamento master: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}


