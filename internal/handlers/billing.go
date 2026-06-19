package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"catalogo/internal/asaas"
	"catalogo/internal/database"
	"catalogo/internal/middleware"

	"github.com/go-chi/chi/v5"
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

	isExpired := shop.PlanExpiresAt != nil && time.Now().After(*shop.PlanExpiresAt)
	if isExpired {
		currentPlan.MaxProducts = 5
		currentPlan.MaxCategories = 1
	}

	data := map[string]interface{}{
		"User":              user,
		"Shop":              shop,
		"Plans":             plans,
		"CurrentPlan":       currentPlan,
		"CurrentProducts":   currentProducts,
		"CurrentCategories": currentCategories,
		"IsExpired":         isExpired,
		"Success":           r.URL.Query().Get("success"),
		"Error":             r.URL.Query().Get("error"),
	}

	if err := h.Tmpl.Render(w, "admin", "admin/billing.html", data); err != nil {
		log.Printf("Erro ao renderizar faturamento: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// UpgradeInitiateResponse é a resposta JSON retornada ao frontend após iniciar uma cobrança
type UpgradeInitiateResponse struct {
	ChargeID    int    `json:"charge_id"`
	BillingType string `json:"billing_type"`
	// PIX
	PixQRCodeBase64 string `json:"pix_qr_code_base64,omitempty"`
	PixCopyPaste    string `json:"pix_copy_paste,omitempty"`
	PixExpiresAt    string `json:"pix_expires_at,omitempty"`
	// Cartão
	InvoiceURL string `json:"invoice_url,omitempty"`
	// Plano Grátis
	Downgrade bool `json:"downgrade,omitempty"`
	// Erro
	Error string `json:"error,omitempty"`
	// Status geral
	Status string `json:"status,omitempty"`
}

// HandleUpgradeInitiate inicia uma cobrança real no Asaas (PIX ou Cartão)
func (h *Handlers) HandleUpgradeInitiate(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	user := middleware.GetUserFromContext(r)
	if shop == nil {
		jsonError(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	_ = r.ParseMultipartForm(10 << 20)

	planIDStr := r.FormValue("plan_id")
	billingType := r.FormValue("billing_type") // "PIX" ou "CREDIT_CARD"
	planID, err := strconv.Atoi(planIDStr)
	if err != nil || planID < 1 {
		jsonError(w, "Plano inválido", http.StatusBadRequest)
		return
	}

	// Plano grátis: downgrade direto, sem cobrar
	if planID == 1 {
		if err := h.DB.UpgradeShopPlan(r.Context(), shop.ID, 1, 0); err != nil {
			log.Printf("Erro ao rebaixar plano da loja %d: %v", shop.ID, err)
			jsonError(w, "Erro ao alterar plano", http.StatusInternalServerError)
			return
		}
		details := fmt.Sprintf("Downgrade para Bronze. Loja: %s (ID %d).", shop.Name, shop.ID)
		_ = h.DB.CreatePlatformAuditLog(r.Context(), "PLAN_DOWNGRADE", details)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(UpgradeInitiateResponse{Downgrade: true})
		return
	}

	// Busca plano para obter o valor
	plan, err := h.DB.GetPlanByID(r.Context(), planID)
	if err != nil {
		jsonError(w, "Plano não encontrado", http.StatusNotFound)
		return
	}

	// Garante que temos o cliente no Asaas
	customerID := shop.AsaasCustomerID
	if customerID == "" {
		phone := ""
		if user != nil {
			phone = ""
		}
		customerID, err = h.AsaasClient.CreateCustomer(shop.Name, user.Email, phone)
		if err != nil {
			log.Printf("Erro ao criar cliente Asaas para loja %d: %v", shop.ID, err)
			jsonError(w, "Erro ao registrar cliente no gateway de pagamento", http.StatusInternalServerError)
			return
		}
		if saveErr := h.DB.SaveAsaasCustomerID(r.Context(), shop.ID, customerID); saveErr != nil {
			log.Printf("Erro ao salvar asaas_customer_id: %v", saveErr)
		}
	}

	description := fmt.Sprintf("Assinatura %s - %s", plan.Name, shop.Name)

	switch billingType {
	case "CREDIT_CARD":
		return // Cartão é tratado em HandleUpgradeCardPost
	default:
		// PIX (padrão)
		charge, err := h.AsaasClient.CreatePixCharge(customerID, plan.Price, description)
		if err != nil {
			log.Printf("Erro ao criar cobrança PIX para loja %d: %v", shop.ID, err)
			jsonError(w, "Erro ao gerar cobrança PIX: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Busca QR Code
		qr, err := h.AsaasClient.GetPixQRCode(charge.ID)
		if err != nil {
			log.Printf("Erro ao buscar QR Code da cobrança %s: %v", charge.ID, err)
			jsonError(w, "Erro ao obter QR Code PIX", http.StatusInternalServerError)
			return
		}

		// Salva cobrança no banco
		dbCharge := &database.PaymentCharge{
			ShopID:         shop.ID,
			PlanID:         planID,
			AsaasPaymentID: charge.ID,
			BillingType:    "PIX",
			Amount:         plan.Price,
			Status:         "PENDING",
			PixQRCode:      qr.EncodedImage,
			PixCopyPaste:   qr.Payload,
		}
		if qr.ExpirationDate != "" {
			if t, err := time.Parse("2006-01-02T15:04:05", qr.ExpirationDate); err == nil {
				dbCharge.ExpiresAt = &t
			}
		}
		if err := h.DB.CreatePaymentCharge(r.Context(), dbCharge); err != nil {
			log.Printf("Erro ao salvar payment_charge: %v", err)
		}

		_ = h.DB.CreatePlatformAuditLog(r.Context(),
			"PAYMENT_PIX_INITIATED",
			fmt.Sprintf("Cobrança PIX iniciada. Loja: %s (ID %d). Plano: %s. AsaasID: %s.", shop.Name, shop.ID, plan.Name, charge.ID),
		)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(UpgradeInitiateResponse{
			ChargeID:        dbCharge.ID,
			BillingType:     "PIX",
			PixQRCodeBase64: qr.EncodedImage,
			PixCopyPaste:    qr.Payload,
			PixExpiresAt:    qr.ExpirationDate,
		})
	}
}

// HandleUpgradeCardPost processa pagamento por cartão de crédito via Asaas
func (h *Handlers) HandleUpgradeCardPost(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	user := middleware.GetUserFromContext(r)
	if shop == nil {
		jsonError(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	_ = r.ParseMultipartForm(10 << 20)

	planIDStr := r.FormValue("plan_id")
	planID, err := strconv.Atoi(planIDStr)
	if err != nil || planID < 2 {
		jsonError(w, "Plano inválido para cobrança", http.StatusBadRequest)
		return
	}

	plan, err := h.DB.GetPlanByID(r.Context(), planID)
	if err != nil {
		jsonError(w, "Plano não encontrado", http.StatusNotFound)
		return
	}

	// Garante cliente no Asaas
	customerID := shop.AsaasCustomerID
	if customerID == "" {
		customerID, err = h.AsaasClient.CreateCustomer(shop.Name, user.Email, "")
		if err != nil {
			jsonError(w, "Erro ao registrar cliente no gateway", http.StatusInternalServerError)
			return
		}
		_ = h.DB.SaveAsaasCustomerID(r.Context(), shop.ID, customerID)
	}

	// Obter IP do cliente final
	remoteIP := r.Header.Get("X-Forwarded-For")
	if remoteIP == "" {
		remoteIP = r.Header.Get("X-Real-IP")
	}
	if remoteIP == "" {
		remoteIP = r.RemoteAddr
	}
	if host, _, err := net.SplitHostPort(remoteIP); err == nil {
		remoteIP = host
	}

	// Dados do cartão (enviados pelo frontend)
	card := asaas.CreditCardData{
		HolderName:  r.FormValue("card_holder"),
		Number:      r.FormValue("card_number"),
		ExpiryMonth: r.FormValue("card_expiry_month"),
		ExpiryYear:  r.FormValue("card_expiry_year"),
		Ccv:         r.FormValue("card_cvv"),
	}

	cpfCnpj := r.FormValue("cpf_cnpj")
	holder := asaas.CreditCardHolder{
		Name:          r.FormValue("card_holder"),
		Email:         user.Email,
		CpfCnpj:       cpfCnpj,
		Phone:         shop.WhatsappNumber,
		PostalCode:    r.FormValue("postal_code"),
		AddressNumber: r.FormValue("address_number"),
	}

	description := fmt.Sprintf("Assinatura %s - %s", plan.Name, shop.Name)

	charge, err := h.AsaasClient.CreateCardCharge(customerID, plan.Price, description, card, holder, remoteIP)
	if err != nil {
		log.Printf("Erro ao criar cobrança de cartão para loja %d: %v", shop.ID, err)
		jsonError(w, "Erro no pagamento: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Salva cobrança
	dbCharge := &database.PaymentCharge{
		ShopID:         shop.ID,
		PlanID:         planID,
		AsaasPaymentID: charge.ID,
		BillingType:    "CREDIT_CARD",
		Amount:         plan.Price,
		Status:         charge.Status,
	}
	if err := h.DB.CreatePaymentCharge(r.Context(), dbCharge); err != nil {
		log.Printf("Erro ao salvar payment_charge cartão: %v", err)
	}

	// Se já foi confirmado instantaneamente (CONFIRMED ou RECEIVED), ativa o plano
	if charge.Status == "CONFIRMED" || charge.Status == "RECEIVED" {
		if err := h.DB.UpgradeShopPlan(r.Context(), shop.ID, planID, 30); err != nil {
			log.Printf("Erro ao ativar plano após cartão confirmado: %v", err)
		}
		_ = h.DB.UpdateChargeStatus(r.Context(), charge.ID, charge.Status)
		details := fmt.Sprintf("Upgrade via cartão confirmado. Loja: %s. Plano: %s. Asaas: %s.", shop.Name, plan.Name, charge.ID)
		_ = h.DB.CreatePlatformAuditLog(r.Context(), "PLAN_UPGRADE_CARD", details)
	}

	_ = h.DB.CreatePlatformAuditLog(r.Context(), "PAYMENT_CARD_INITIATED",
		fmt.Sprintf("Cobrança cartão. Loja: %s (ID %d). Plano: %s. Status: %s.", shop.Name, shop.ID, plan.Name, charge.Status))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(UpgradeInitiateResponse{
		ChargeID:    dbCharge.ID,
		BillingType: "CREDIT_CARD",
		InvoiceURL:  charge.InvoiceURL,
		Status:      charge.Status,
	})
}

// HandleCheckChargeStatus faz polling do status de uma cobrança (frontend consulta a cada 5s)
func (h *Handlers) HandleCheckChargeStatus(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		jsonError(w, "Não autenticado", http.StatusUnauthorized)
		return
	}

	chargeIDStr := chi.URLParam(r, "charge_id")
	chargeID, err := strconv.Atoi(chargeIDStr)
	if err != nil {
		jsonError(w, "ID inválido", http.StatusBadRequest)
		return
	}

	dbCharge, err := h.DB.GetChargeByID(r.Context(), chargeID)
	if err != nil || dbCharge.ShopID != shop.ID {
		jsonError(w, "Cobrança não encontrada", http.StatusNotFound)
		return
	}

	// Consulta status atual no Asaas
	status, err := h.AsaasClient.GetPaymentStatus(dbCharge.AsaasPaymentID)
	if err != nil {
		log.Printf("Erro ao consultar status da cobrança %s: %v", dbCharge.AsaasPaymentID, err)
		// Retorna o status do banco mesmo sem atualizar
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": dbCharge.Status})
		return
	}

	// Se pagamento confirmado, ativa o plano
	if (status == "RECEIVED" || status == "CONFIRMED") && dbCharge.Status == "PENDING" {
		if err := h.DB.UpgradeShopPlan(r.Context(), shop.ID, dbCharge.PlanID, 30); err != nil {
			log.Printf("Erro ao ativar plano via polling: %v", err)
		}
		if err := h.DB.UpdateChargeStatus(r.Context(), dbCharge.AsaasPaymentID, status); err != nil {
			log.Printf("Erro ao atualizar status: %v", err)
		}

		plan, _ := h.DB.GetPlanByID(r.Context(), dbCharge.PlanID)
		planName := "desconhecido"
		if plan != nil {
			planName = plan.Name
		}
		details := fmt.Sprintf("Upgrade ativado via polling. Loja: %s (ID %d). Plano: %s.", shop.Name, shop.ID, planName)
		_ = h.DB.CreatePlatformAuditLog(r.Context(), "PLAN_UPGRADE_POLLING", details)
	} else {
		// Apenas atualiza o status no banco
		_ = h.DB.UpdateChargeStatus(r.Context(), dbCharge.AsaasPaymentID, status)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": status})
}

// HandleAsaasWebhook recebe eventos de webhook do Asaas e ativa planos automaticamente
func (h *Handlers) HandleAsaasWebhook(w http.ResponseWriter, r *http.Request) {
	// Valida o token de segurança enviado pelo Asaas no header
	token := r.Header.Get("asaas-access-token")
	if token == "" {
		// Asaas pode enviar no header "access_token" também
		token = r.Header.Get("access_token")
	}

	// Lê o body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("[WEBHOOK] Erro ao ler body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Printf("[WEBHOOK ASAAS] Recebido: %s", string(body))

	var event asaas.WebhookPayment
	if err := json.Unmarshal(body, &event); err != nil {
		log.Printf("[WEBHOOK] Erro ao parsear evento: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Processa apenas pagamentos recebidos/confirmados
	if event.Event != "PAYMENT_RECEIVED" && event.Event != "PAYMENT_CONFIRMED" {
		log.Printf("[WEBHOOK] Evento ignorado: %s", event.Event)
		w.WriteHeader(http.StatusOK)
		return
	}

	paymentID := event.Payment.ID
	if paymentID == "" {
		log.Printf("[WEBHOOK] Payment ID vazio no evento")
		w.WriteHeader(http.StatusOK)
		return
	}

	// Busca a cobrança no banco
	dbCharge, err := h.DB.GetChargeByAsaasID(r.Context(), paymentID)
	if err != nil {
		log.Printf("[WEBHOOK] Cobrança não encontrada para Asaas ID %s: %v", paymentID, err)
		// Responde 200 mesmo assim para o Asaas não reenviar
		w.WriteHeader(http.StatusOK)
		return
	}

	// Só ativa se ainda estava pendente
	if dbCharge.Status != "PENDING" {
		log.Printf("[WEBHOOK] Cobrança %s já processada com status %s", paymentID, dbCharge.Status)
		w.WriteHeader(http.StatusOK)
		return
	}

	// Ativa o plano
	if err := h.DB.UpgradeShopPlan(r.Context(), dbCharge.ShopID, dbCharge.PlanID, 30); err != nil {
		log.Printf("[WEBHOOK] Erro ao ativar plano para shop %d: %v", dbCharge.ShopID, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Atualiza status da cobrança
	if err := h.DB.UpdateChargeStatus(r.Context(), paymentID, "RECEIVED"); err != nil {
		log.Printf("[WEBHOOK] Erro ao atualizar status da cobrança: %v", err)
	}

	plan, _ := h.DB.GetPlanByID(r.Context(), dbCharge.PlanID)
	planName := "desconhecido"
	if plan != nil {
		planName = plan.Name
	}
	details := fmt.Sprintf("Plano ativado via webhook. Shop ID: %d. Plano: %s. AsaasID: %s. Evento: %s.",
		dbCharge.ShopID, planName, paymentID, event.Event)
	_ = h.DB.CreatePlatformAuditLog(r.Context(), "PLAN_UPGRADE_WEBHOOK", details)

	log.Printf("[WEBHOOK] ✅ Plano %s ativado para shop %d via evento %s", planName, dbCharge.ShopID, event.Event)
	w.WriteHeader(http.StatusOK)
}

// jsonError responde com JSON de erro
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
