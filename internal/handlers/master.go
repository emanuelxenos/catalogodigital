package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

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

// HandleMasterDashboard renderiza o painel de controle mestre do SaaS
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
		}
	}

	// 2. Busca lista de lojas
	shops, err := h.DB.ListShopsWithOwners(r.Context())
	if err != nil {
		log.Printf("[MASTER] Erro ao listar lojas: %v", err)
		shops = []database.ShopWithOwner{}
	}

	// 3. Busca configurações globais
	configs, err := h.DB.GetPlatformConfigs(r.Context())
	if err != nil {
		log.Printf("[MASTER] Erro ao carregar configurações: %v", err)
		configs = map[string]string{
			"platform_name":           "Catálogo Digital SaaS",
			"maintenance_mode":        "false",
			"global_subscription_fee": "49.90",
			"support_whatsapp":        "5511999999999",
		}
	}

	// 4. Busca logs de auditoria
	auditLogs, err := h.DB.ListPlatformAuditLogs(r.Context())
	if err != nil {
		log.Printf("[MASTER] Erro ao carregar logs de auditoria: %v", err)
		auditLogs = []database.PlatformAuditLog{}
	}

	data := map[string]interface{}{
		"User":      user,
		"Metrics":   metrics,
		"Shops":     shops,
		"Configs":   configs,
		"AuditLogs": auditLogs,
	}

	// Renderiza usando o layout exclusivo do super admin ("super")
	if err := h.Tmpl.Render(w, "super", "master/dashboard.html", data); err != nil {
		log.Printf("[MASTER] Erro ao renderizar master dashboard: %v", err)
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
