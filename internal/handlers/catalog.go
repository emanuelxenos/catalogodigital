package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"catalogo/internal/database"

	"github.com/go-chi/chi/v5"
)

// HandleCatalog renderiza o catálogo público de uma loja
func (h *Handlers) HandleCatalog(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	// Busca a loja pelo slug
	shop, err := h.DB.GetShopBySlug(r.Context(), slug)
	if err != nil {
		log.Printf("Loja não encontrada: %s - %v", slug, err)
		http.NotFound(w, r)
		return
	}

	// Verifica se a loja está ativa
	if !shop.IsActive {
		data := map[string]interface{}{
			"Shop": shop,
		}
		if err := h.Tmpl.RenderPage(w, "catalog/inactive.html", data); err != nil {
			log.Printf("Erro ao renderizar pagina inativa: %v", err)
			http.Error(w, "Erro interno", http.StatusInternalServerError)
		}
		return
	}

	// Busca categorias da loja
	categories, err := h.DB.ListCategoriesByShop(r.Context(), shop.ID)
	if err != nil {
		log.Printf("Erro ao listar categorias: %v", err)
	}

	// Busca produtos disponíveis
	products, err := h.DB.ListAvailableProductsByShop(r.Context(), shop.ID)
	if err != nil {
		log.Printf("Erro ao listar produtos: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
		return
	}

	// Verifica se a loja está aberta (apenas para planos Ouro e Diamante)
	isOpen := true
	if shop.PlanID == 3 || shop.PlanID == 4 {
		isOpen = IsShopOpen(shop.BusinessHours)
	}

	var parsedHours map[string]map[string]string
	if shop.BusinessHours != nil && *shop.BusinessHours != "" && *shop.BusinessHours != "null" {
		if err := json.Unmarshal([]byte(*shop.BusinessHours), &parsedHours); err != nil {
			log.Printf("Erro ao parsear BusinessHours: %v", err)
		}
	}

	var banners []database.ShopBanner
	if shop.PlanID == 4 {
		var err error
		banners, err = h.DB.ListShopBanners(r.Context(), shop.ID)
		if err != nil {
			log.Printf("Erro ao buscar banners para o catalogo: %v", err)
		}
	}

	data := map[string]interface{}{
		"Shop":          shop,
		"Categories":    categories,
		"Products":      products,
		"IsOpen":        isOpen,
		"BusinessHours": parsedHours,
		"Banners":       banners,
	}

	if err := h.Tmpl.Render(w, "base", "catalog/index.html", data); err != nil {
		log.Printf("Erro ao renderizar catálogo: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleProductsByCategory retorna produtos filtrados por categoria (endpoint HTMX)
func (h *Handlers) HandleProductsByCategory(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		http.NotFound(w, r)
		return
	}

	shop, err := h.DB.GetShopBySlug(r.Context(), slug)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	if !shop.IsActive {
		http.Error(w, "Loja inativa", http.StatusForbidden)
		return
	}

	categoriaStr := r.URL.Query().Get("categoria")

	var data map[string]interface{}

	if categoriaStr == "" || categoriaStr == "0" {
		// Sem filtro — retorna todos os produtos disponíveis
		products, err := h.DB.ListAvailableProductsByShop(r.Context(), shop.ID)
		if err != nil {
			http.Error(w, "Erro interno", http.StatusInternalServerError)
			return
		}
		data = map[string]interface{}{
			"Shop":     shop,
			"Products": products,
		}
	} else {
		categoriaID, err := strconv.Atoi(categoriaStr)
		if err != nil {
			http.Error(w, "Categoria inválida", http.StatusBadRequest)
			return
		}

		products, err := h.DB.ListProductsByCategory(r.Context(), shop.ID, categoriaID)
		if err != nil {
			http.Error(w, "Erro interno", http.StatusInternalServerError)
			return
		}
		data = map[string]interface{}{
			"Shop":     shop,
			"Products": products,
		}
	}

	if err := h.Tmpl.RenderPartial(w, "catalog/products.html", data); err != nil {
		log.Printf("Erro ao renderizar produtos: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleValidateCoupon valida se um cupom existe e está ativo para a loja
func (h *Handlers) HandleValidateCoupon(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	code := chi.URLParam(r, "code")

	w.Header().Set("Content-Type", "application/json")

	if slug == "" || code == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":   false,
			"message": "Dados de cupom inválidos",
		})
		return
	}

	shop, err := h.DB.GetShopBySlug(r.Context(), slug)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":   false,
			"message": "Loja não encontrada",
		})
		return
	}

	if !shop.IsActive {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":   false,
			"message": "Loja inativa",
		})
		return
	}

	// Bloqueia cupons no plano Bronze (ID 1)
	if shop.PlanID == 1 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":   false,
			"message": "Cupons de desconto não são aceitos nesta loja.",
		})
		return
	}

	coupon, err := h.DB.GetCouponByCode(r.Context(), shop.ID, code)
	if err != nil {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid":   false,
			"message": "Cupom inválido ou expirado",
		})
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"valid": true,
		"code":  coupon.Code,
		"type":  coupon.Type,
		"value": coupon.Value,
	})
}

type CheckoutItem struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Price   float64 `json:"price"`
	Qty     int    `json:"qty"`
	Note    string `json:"note"`
	Options string `json:"options"` // JSON string contendo opções selecionadas
}

type CheckoutRequest struct {
	CustomerName   string         `json:"customerName"`
	CustomerPhone  string         `json:"customerPhone"`
	CustomerEmail  string         `json:"customerEmail"`
	DeliveryMethod string         `json:"deliveryMethod"` // "entrega" ou "retirada"
	Address        string         `json:"address"`
	PaymentMethod  string         `json:"paymentMethod"`
	CouponCode     string         `json:"couponCode"`
	Items          []CheckoutItem `json:"items"`
}

type ChosenChoice struct {
	Name        string  `json:"name"`
	PriceAdjust float64 `json:"price_adjust"`
}

// HandleCheckout processa a finalização do pedido, valida, grava no banco e retorna a URL do WhatsApp
func (h *Handlers) HandleCheckout(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	w.Header().Set("Content-Type", "application/json")

	shop, err := h.DB.GetShopBySlug(r.Context(), slug)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Loja não encontrada",
		})
		return
	}

	if !shop.IsActive {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Loja inativa",
		})
		return
	}

	// 1. Valida se a loja está aberta (apenas para planos Ouro e Diamante)
	if (shop.PlanID == 3 || shop.PlanID == 4) && !IsShopOpen(shop.BusinessHours) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "A loja está fechada para pedidos no momento. Por favor, consulte nosso horário de funcionamento.",
		})
		return
	}

	var req CheckoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Dados do checkout inválidos",
		})
		return
	}

	// Validações básicas
	if req.CustomerName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Informe seu nome",
		})
		return
	}
	cleanedPhone := cleanWhatsAppNumber(req.CustomerPhone)
	if len(cleanedPhone) < 12 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Informe o WhatsApp completo com DDD (ex: 11999999999)",
		})
		return
	}
	req.CustomerPhone = cleanedPhone
	if req.DeliveryMethod == "entrega" && req.Address == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Informe o endereço de entrega",
		})
		return
	}
	if req.PaymentMethod == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Selecione uma forma de pagamento",
		})
		return
	}
	if len(req.Items) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Seu carrinho está vazio",
		})
		return
	}

	var subtotal float64
	var orderItemsToSave []database.OrderItem

	// 2. Valida produtos e calcula subtotal
	for _, item := range req.Items {
		p, err := h.DB.GetProduct(r.Context(), item.ID, shop.ID)
		if err != nil || !p.IsAvailable {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": fmt.Sprintf("Produto indisponível ou inexistente: %s", item.Name),
			})
			return
		}

		// Calcula preço adicional baseado nas opções
		var priceAdjust float64
		if item.Options != "" {
			var chosen map[string]json.RawMessage
			if err := json.Unmarshal([]byte(item.Options), &chosen); err == nil {
				for _, raw := range chosen {
					// Tenta como lista (multi-escolha)
					var list []ChosenChoice
					if err := json.Unmarshal(raw, &list); err == nil {
						for _, c := range list {
							priceAdjust += c.PriceAdjust
						}
					} else {
						// Tenta como objeto único (single-choice)
						var c ChosenChoice
						if err := json.Unmarshal(raw, &c); err == nil {
							priceAdjust += c.PriceAdjust
						}
					}
				}
			}
		}

		finalItemPrice := p.Price + priceAdjust
		itemSubtotal := finalItemPrice * float64(item.Qty)
		subtotal += itemSubtotal

		var optPtr *string
		if item.Options != "" {
			optPtr = &item.Options
		}

		orderItemsToSave = append(orderItemsToSave, database.OrderItem{
			ProductID: &p.ID,
			Name:      p.Name,
			Price:     finalItemPrice,
			Qty:       item.Qty,
			Note:      item.Note,
			Options:   optPtr,
		})
	}

	// 3. Aplica cupom de desconto (apenas para planos diferentes de Bronze/1)
	var discount float64
	if req.CouponCode != "" && shop.PlanID != 1 {
		coupon, err := h.DB.GetCouponByCode(r.Context(), shop.ID, req.CouponCode)
		if err == nil {
			if coupon.Type == "percentage" {
				discount = subtotal * (coupon.Value / 100.0)
			} else if coupon.Type == "fixed" {
				discount = coupon.Value
			}
			if discount > subtotal {
				discount = subtotal
			}
		}
	}

	// 4. Calcula taxa de entrega
	var deliveryFee float64
	if req.DeliveryMethod == "entrega" {
		deliveryFee = shop.DeliveryFee
	}

	total := subtotal - discount + deliveryFee
	if total < 0 {
		total = 0
	}

	// 5. Salva o pedido no banco
	dbOrder := &database.Order{
		ShopID:         shop.ID,
		CustomerName:   req.CustomerName,
		CustomerPhone:  req.CustomerPhone,
		CustomerEmail:  strings.TrimSpace(req.CustomerEmail),
		DeliveryMethod: req.DeliveryMethod,
		Address:        req.Address,
		PaymentMethod:  req.PaymentMethod,
		CouponCode:     req.CouponCode,
		Discount:       discount,
		Subtotal:       subtotal,
		Total:          total,
		Status:         "Pendente",
	}

	if err := h.DB.CreateOrder(r.Context(), dbOrder); err != nil {
		log.Printf("Erro ao salvar pedido no banco: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "Erro interno ao processar pedido",
		})
		return
	}

	for _, item := range orderItemsToSave {
		item.OrderID = dbOrder.ID
		if err := h.DB.CreateOrderItem(r.Context(), &item); err != nil {
			log.Printf("Erro ao salvar item de pedido %d: %v", dbOrder.ID, err)
		}
	}

	// Busca o lojista (User) dono da loja para obter o e-mail de envio
	user, err := h.DB.GetUserByID(r.Context(), shop.UserID)
	if err != nil {
		log.Printf("Erro ao buscar lojista pelo ID %d para enviar e-mail: %v", shop.UserID, err)
	} else if user != nil && user.Email != "" {
		// Envia a notificação por e-mail em segundo plano
		go func(s *database.Shop, o *database.Order, items []database.OrderItem, email string) {
			err := h.Mailer.SendOrderNotification(s, o, items, email)
			if err != nil {
				log.Printf("[Erro Mailer] Falha ao enviar e-mail de pedido para %s: %v", email, err)
			} else {
				log.Printf("[Mailer] E-mail de notificação de novo pedido enviado para %s", email)
			}
		}(shop, dbOrder, orderItemsToSave, user.Email)
	}

	// 6. Gera a mensagem do WhatsApp e monta URL
	var msg strings.Builder
	msg.WriteString("🛒 *NOVO PEDIDO*\n")
	msg.WriteString("━━━━━━━━━━━━━━━\n\n")

	for idx, item := range orderItemsToSave {
		msg.WriteString(fmt.Sprintf("*%d.* %s\n", idx+1, item.Name))
		if item.Options != nil && *item.Options != "" && *item.Options != "{}" {
			// Formata opções amigavelmente para o texto do WhatsApp
			var chosen map[string]json.RawMessage
			var optTexts []string
			if err := json.Unmarshal([]byte(*item.Options), &chosen); err == nil {
				for optKey, raw := range chosen {
					var list []ChosenChoice
					if err := json.Unmarshal(raw, &list); err == nil {
						var subOpts []string
						for _, c := range list {
							subOpts = append(subOpts, c.Name)
						}
						optTexts = append(optTexts, fmt.Sprintf("%s: %s", optKey, strings.Join(subOpts, ", ")))
					} else {
						var c ChosenChoice
						if err := json.Unmarshal(raw, &c); err == nil {
							optTexts = append(optTexts, fmt.Sprintf("%s: %s", optKey, c.Name))
						}
					}
				}
			}
			if len(optTexts) > 0 {
				msg.WriteString(fmt.Sprintf("   ⚙️ _%s_\n", strings.Join(optTexts, " | ")))
			}
		}
		if item.Note != "" {
			msg.WriteString(fmt.Sprintf("   📝 _Obs: %s_\n", item.Note))
		}
		msg.WriteString(fmt.Sprintf("   Qtd: %d x %s\n", item.Qty, formatBRL(item.Price)))
		msg.WriteString(fmt.Sprintf("   Subtotal: %s\n\n", formatBRL(item.Price*float64(item.Qty))))
	}

	msg.WriteString("━━━━━━━━━━━━━━━\n")
	msg.WriteString(fmt.Sprintf("💰 *Subtotal:* %s\n", formatBRL(subtotal)))
	if discount > 0 {
		msg.WriteString(fmt.Sprintf("🎟️ *Desconto:* -%s\n", formatBRL(discount)))
	}
	if req.DeliveryMethod == "entrega" {
		msg.WriteString(fmt.Sprintf("🛵 *Taxa de Entrega:* %s\n", formatBRL(deliveryFee)))
	}
	msg.WriteString(fmt.Sprintf("✨ *TOTAL: %s*\n", formatBRL(total)))
	msg.WriteString("━━━━━━━━━━━━━━━\n\n")

	msg.WriteString(fmt.Sprintf("👤 *Nome:* %s\n", req.CustomerName))
	if req.DeliveryMethod == "entrega" {
		msg.WriteString("🛵 *Método:* Entrega\n")
		msg.WriteString(fmt.Sprintf("📍 *Endereço:* %s\n", req.Address))
	} else {
		msg.WriteString("🏪 *Método:* Retirada no local\n")
	}

	paymentLabels := map[string]string{
		"pix":      "⚡ Pix",
		"cartao":   "💳 Cartão de Crédito/Débito",
		"dinheiro": "💵 Dinheiro",
	}
	payMethod := req.PaymentMethod
	if lbl, ok := paymentLabels[req.PaymentMethod]; ok {
		payMethod = lbl
	}
	msg.WriteString(fmt.Sprintf("💳 *Pagamento:* %s\n", payMethod))

	encodedMsg := strings.ReplaceAll(msg.String(), " ", "%20")
	encodedMsg = strings.ReplaceAll(encodedMsg, "\n", "%0A")
	whatsappURL := fmt.Sprintf("https://wa.me/%s?text=%s", shop.WhatsappNumber, encodedMsg)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"success":      true,
		"order_id":     dbOrder.ID,
		"whatsapp_url": whatsappURL,
	})
}

// formatBRL formata valores para Real Brasileiro (R$)
func formatBRL(val float64) string {
	s := fmt.Sprintf("%.2f", val)
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return "R$ " + s[:i] + "," + s[i+1:]
		}
	}
	return "R$ " + s
}

// DayHours representa os horários de abertura e fechamento de um dia
type DayHours struct {
	Open  string `json:"open"`
	Close string `json:"close"`
}

// IsShopOpen valida os horários comerciais da loja
func IsShopOpen(businessHours *string) bool {
	if businessHours == nil || *businessHours == "" || *businessHours == "null" {
		log.Println("[IsShopOpen] business_hours é nulo/vazio → retornando aberto por padrão")
		return true // Aberto por padrão
	}

	hours := make(map[string]DayHours)

	if err := json.Unmarshal([]byte(*businessHours), &hours); err != nil {
		log.Printf("[IsShopOpen] ERRO ao fazer parse do JSON de horários: %v | JSON: %s", err, *businessHours)
		return false // Conservador: se não consegue ler o horário, considera fechado
	}

	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		loc = time.FixedZone("UTC-3", -3*60*60)
	}
	now := time.Now().In(loc)
	weekdays := []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}
	dayKey := weekdays[now.Weekday()]

	dayConfig, exists := hours[dayKey]
	log.Printf("[IsShopOpen] dia=%s | config encontrado=%v | open=%s close=%s", dayKey, exists, dayConfig.Open, dayConfig.Close)

	if !exists || dayConfig.Open == "" || dayConfig.Close == "" {
		log.Printf("[IsShopOpen] Dia '%s' sem horário configurado → FECHADO", dayKey)
		return false
	}

	currentHM := now.Format("15:04")
	isOpen := currentHM >= dayConfig.Open && currentHM <= dayConfig.Close
	log.Printf("[IsShopOpen] agora=%s | range=%s-%s | aberto=%v", currentHM, dayConfig.Open, dayConfig.Close, isOpen)
	return isOpen
}

