package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"catalogo/internal/database"
	"catalogo/internal/middleware"
)

// HandleReportsPage renderiza o painel de relatórios analíticos de vendas
func (h *Handlers) HandleReportsPage(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
		return
	}

	isLocked := shop.PlanID == 1 || shop.PlanID == 2
	isPDFLocked := shop.PlanID == 3 // Ouro tem acesso aos gráficos, mas não ao PDF

	var metrics map[string]float64
	var bestSellers []database.BestSeller
	var paymentStats []database.PaymentMethodStat
	var dailySales []database.DailySales
	var dailyJSON string = "[]"
	var paymentJSON string = "[]"
	var err error

	if !isLocked {
		// 1. Métricas básicas de vendas (Faturamento, Pedidos, Ticket Médio)
		metrics, err = h.DB.GetOrderMetrics(r.Context(), shop.ID)
		if err != nil {
			log.Printf("[REPORTS] Erro ao buscar métricas: %v", err)
		}

		// 2. Ranking de mais vendidos
		bestSellers, err = h.DB.GetBestSellingProducts(r.Context(), shop.ID, 5)
		if err != nil {
			log.Printf("[REPORTS] Erro ao buscar mais vendidos: %v", err)
		}

		// 3. Métodos de pagamento
		paymentStats, err = h.DB.GetSalesByPaymentMethod(r.Context(), shop.ID)
		if err == nil {
			pBytes, _ := json.Marshal(paymentStats)
			paymentJSON = string(pBytes)
		} else {
			log.Printf("[REPORTS] Erro ao buscar métodos de pagamento: %v", err)
		}

		// 4. Vendas diárias últimos 30 dias para o gráfico de linha
		dailySales, err = h.DB.GetDailySalesLast30Days(r.Context(), shop.ID)
		if err == nil {
			dBytes, _ := json.Marshal(dailySales)
			dailyJSON = string(dBytes)
		} else {
			log.Printf("[REPORTS] Erro ao buscar faturamento 30 dias: %v", err)
		}
	}

	data := map[string]interface{}{
		"User":         user,
		"Shop":         shop,
		"IsLocked":     isLocked,
		"IsPDFLocked":  isPDFLocked,
		"Metrics":      metrics,
		"BestSellers":  bestSellers,
		"PaymentStats": paymentStats,
		"DailySales":   dailySales,
		"DailyJSON":    dailyJSON,
		"PaymentJSON":  paymentJSON,
	}

	if err := h.Tmpl.Render(w, "admin", "admin/reports.html", data); err != nil {
		log.Printf("[REPORTS] Erro ao renderizar tela de relatórios: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandlePrintReports renderiza uma view limpa otimizada para salvar como PDF/Imprimir (Exclusivo Diamante)
func (h *Handlers) HandlePrintReports(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Error(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	// Trava: Apenas Diamante (ID 4) pode exportar PDF/Imprimir relatórios
	if shop.PlanID != 4 {
		http.Redirect(w, r, "/admin/relatorios?error=Exportação em PDF é exclusiva do plano Diamante.", http.StatusSeeOther)
		return
	}

	metrics, err := h.DB.GetOrderMetrics(r.Context(), shop.ID)
	if err != nil {
		log.Printf("[REPORTS] Erro ao buscar métricas para impressao: %v", err)
	}

	bestSellers, err := h.DB.GetBestSellingProducts(r.Context(), shop.ID, 10)
	if err != nil {
		log.Printf("[REPORTS] Erro ao buscar mais vendidos para impressao: %v", err)
	}

	paymentStats, err := h.DB.GetSalesByPaymentMethod(r.Context(), shop.ID)
	if err != nil {
		log.Printf("[REPORTS] Erro ao buscar pagamentos para impressao: %v", err)
	}

	dailySales, err := h.DB.GetDailySalesLast30Days(r.Context(), shop.ID)
	if err != nil {
		log.Printf("[REPORTS] Erro ao buscar vendas para impressao: %v", err)
	}

	data := map[string]interface{}{
		"Shop":         shop,
		"Metrics":      metrics,
		"BestSellers":  bestSellers,
		"PaymentStats": paymentStats,
		"DailySales":   dailySales,
		"Now":          time.Now(),
	}

	// Renderiza standalone sem o painel admin (folha de impressão limpa)
	if err := h.Tmpl.RenderPage(w, "admin/print_reports.html", data); err != nil {
		log.Printf("[REPORTS] Erro ao renderizar folha de impressao: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}
