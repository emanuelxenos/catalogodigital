package handlers

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"catalogo/internal/database"
	"catalogo/internal/middleware"
)

// HandleShopBilling renderiza a página de faturamento e planos do lojista
func (h *Handlers) HandleShopBilling(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
		return
	}

	// 1. Carrega dados de uso (produtos e categorias atuais)
	currentProducts, currentCategories, err := h.DB.GetShopUsage(r.Context(), shop.ID)
	if err != nil {
		log.Printf("Erro ao buscar uso da loja %d: %v", shop.ID, err)
	}

	// 2. Carrega todos os planos cadastrados para exibir no grid
	rows, err := h.DB.Pool.Query(r.Context(), "SELECT id, name, price, max_products, max_categories, features::text FROM plans ORDER BY id ASC")
	var plans []database.Plan
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p database.Plan
			if err := rows.Scan(&p.ID, &p.Name, &p.Price, &p.MaxProducts, &p.MaxCategories, &p.Features); err == nil {
				plans = append(plans, p)
			}
		}
	} else {
		log.Printf("Erro ao carregar planos: %v", err)
	}

	// 3. Carrega o plano atual da loja
	currentPlan, err := h.DB.GetPlanByID(r.Context(), shop.PlanID)
	if err != nil {
		log.Printf("Erro ao buscar plano atual %d: %v", shop.PlanID, err)
		currentPlan = &database.Plan{Name: "Bronze (Grátis)"}
	}

	data := map[string]interface{}{
		"User":              user,
		"Shop":              shop,
		"Plans":             plans,
		"CurrentPlan":       currentPlan,
		"CurrentProducts":   currentProducts,
		"CurrentCategories": currentCategories,
		"Success":           r.URL.Query().Get("success"),
		"Error":             r.URL.Query().Get("error"),
	}

	if err := h.Tmpl.Render(w, "admin", "admin/billing.html", data); err != nil {
		log.Printf("Erro ao renderizar faturamento: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleUpgradeSimulatorPost simula um pagamento bem-sucedido e altera o plano do lojista
func (h *Handlers) HandleUpgradeSimulatorPost(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Error(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	planIDStr := r.FormValue("plan_id")
	planID, err := strconv.Atoi(planIDStr)
	if err != nil {
		http.Error(w, "Plano inválido", http.StatusBadRequest)
		return
	}

	// 1. Busca detalhes do plano selecionado
	plan, err := h.DB.GetPlanByID(r.Context(), planID)
	if err != nil {
		http.Error(w, "Plano não encontrado", http.StatusNotFound)
		return
	}

	// 2. Define validade (30 dias para pagos, indefinido para Bronze/Grátis)
	days := 30
	if planID == 1 {
		days = 0
	}

	// 3. Aplica upgrade no banco
	if err := h.DB.UpgradeShopPlan(r.Context(), shop.ID, planID, days); err != nil {
		log.Printf("Erro ao atualizar plano da loja %d: %v", shop.ID, err)
		http.Error(w, "Erro ao processar upgrade", http.StatusInternalServerError)
		return
	}

	// 4. Registra ação no Audit Log SaaS
	details := fmt.Sprintf("Upgrade efetuado com sucesso via checkout simulado. Loja: %s (ID %d). Novo Plano: %s.", 
		shop.Name, shop.ID, plan.Name)
	if err := h.DB.CreatePlatformAuditLog(r.Context(), "PLAN_UPGRADE", details); err != nil {
		log.Printf("Erro ao criar log de auditoria SaaS: %v", err)
	}

	// Força recarregamento total da página lojista para atualizar o plano
	w.Header().Set("HX-Refresh", "true")
	w.WriteHeader(http.StatusOK)
}
