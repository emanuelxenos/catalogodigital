package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"catalogo/internal/database"
	"catalogo/internal/middleware"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
)

// ==================== AUTH ====================

// HandleLoginPage renderiza o formulário de login
func (h *Handlers) HandleLoginPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Error": r.URL.Query().Get("error"),
	}
	if err := h.Tmpl.RenderPage(w, "admin/login.html", data); err != nil {
		log.Printf("Erro ao renderizar login: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleLoginPost processa o login
func (h *Handlers) HandleLoginPost(w http.ResponseWriter, r *http.Request) {
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	if email == "" || password == "" {
		http.Redirect(w, r, "/admin/login?error=Preencha todos os campos", http.StatusSeeOther)
		return
	}

	user, err := h.DB.GetUserByEmail(r.Context(), email)
	if err != nil {
		http.Redirect(w, r, "/admin/login?error=Email ou senha incorretos", http.StatusSeeOther)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		http.Redirect(w, r, "/admin/login?error=Email ou senha incorretos", http.StatusSeeOther)
		return
	}

	// Cria sessão
	session, err := h.DB.CreateSession(r.Context(), user.ID)
	if err != nil {
		log.Printf("Erro ao criar sessão: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// HandleRegisterPage renderiza o formulário de registro
func (h *Handlers) HandleRegisterPage(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Error": r.URL.Query().Get("error"),
	}
	if err := h.Tmpl.RenderPage(w, "admin/register.html", data); err != nil {
		log.Printf("Erro ao renderizar registro: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleRegisterPost processa o registro de novo usuário + loja
func (h *Handlers) HandleRegisterPost(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")
	shopName := strings.TrimSpace(r.FormValue("shop_name"))
	whatsapp := cleanWhatsAppNumber(strings.TrimSpace(r.FormValue("whatsapp")))

	if name == "" || email == "" || password == "" || shopName == "" || whatsapp == "" {
		http.Redirect(w, r, "/admin/register?error=Preencha todos os campos", http.StatusSeeOther)
		return
	}

	if len(password) < 6 {
		http.Redirect(w, r, "/admin/register?error=A senha deve ter pelo menos 6 caracteres", http.StatusSeeOther)
		return
	}

	// Hash da senha
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Printf("Erro ao gerar hash: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
		return
	}

	// Cria usuário
	user, err := h.DB.CreateUser(r.Context(), name, email, string(hash))
	if err != nil {
		log.Printf("Erro ao criar usuário: %v", err)
		http.Redirect(w, r, "/admin/register?error=Este email já está cadastrado", http.StatusSeeOther)
		return
	}

	// Gera slug a partir do nome da loja
	slug := generateSlug(shopName)

	// Cria loja
	shop := &database.Shop{
		UserID:         user.ID,
		Name:           shopName,
		Slug:           slug,
		WhatsappNumber: whatsapp,
		PrimaryColor:   "#8B5CF6",
	}
	if err := h.DB.CreateShop(r.Context(), shop); err != nil {
		log.Printf("Erro ao criar loja: %v", err)
		http.Redirect(w, r, "/admin/register?error=Erro ao criar loja. Tente outro nome.", http.StatusSeeOther)
		return
	}

	// Cria sessão e loga automaticamente
	session, err := h.DB.CreateSession(r.Context(), user.ID)
	if err != nil {
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    session.ID,
		Path:     "/",
		Expires:  session.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// HandleLogout encerra a sessão
func (h *Handlers) HandleLogout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_id")
	if err == nil {
		_ = h.DB.DeleteSession(r.Context(), cookie.Value)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// ==================== DASHBOARD ====================

// HandleDashboard renderiza o painel principal
func (h *Handlers) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
		return
	}

	productCount, _ := h.DB.CountProductsByShop(r.Context(), shop.ID)
	categoryCount, _ := h.DB.CountCategoriesByShop(r.Context(), shop.ID)

	metrics, err := h.DB.GetOrderMetrics(r.Context(), shop.ID)
	if err != nil {
		log.Printf("Erro ao obter métricas de pedidos: %v", err)
		metrics = map[string]float64{
			"revenue_month":  0,
			"total_orders":   0,
			"pending_orders": 0,
			"average_ticket": 0,
		}
	}

	data := map[string]interface{}{
		"User":          user,
		"Shop":          shop,
		"ProductCount":  productCount,
		"CategoryCount": categoryCount,
		"Metrics":       metrics,
	}

	if err := h.Tmpl.Render(w, "admin", "admin/dashboard.html", data); err != nil {
		log.Printf("Erro ao renderizar dashboard: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// ==================== PRODUCTS ====================

// HandleProducts lista os produtos no painel admin
func (h *Handlers) HandleProducts(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
		return
	}

	products, err := h.DB.ListProductsByShop(r.Context(), shop.ID)
	if err != nil {
		log.Printf("Erro ao listar produtos: %v", err)
	}

	categories, err := h.DB.ListCategoriesByShop(r.Context(), shop.ID)
	if err != nil {
		log.Printf("Erro ao listar categorias: %v", err)
	}

	data := map[string]interface{}{
		"User":       user,
		"Shop":       shop,
		"Products":   products,
		"Categories": categories,
		"Success":    r.URL.Query().Get("success"),
		"Error":      r.URL.Query().Get("error"),
	}

	if err := h.Tmpl.Render(w, "admin", "admin/products.html", data); err != nil {
		log.Printf("Erro ao renderizar produtos: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleCreateProduct cria um novo produto
func (h *Handlers) HandleCreateProduct(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Error(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10MB max
		http.Redirect(w, r, "/admin/produtos?error=Erro ao processar formulário", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	description := strings.TrimSpace(r.FormValue("description"))
	priceStr := strings.TrimSpace(r.FormValue("price"))
	categoryStr := r.FormValue("category_id")
	optionsStr := strings.TrimSpace(r.FormValue("options"))

	if name == "" || priceStr == "" {
		http.Redirect(w, r, "/admin/produtos?error=Nome e preço são obrigatórios", http.StatusSeeOther)
		return
	}

	// Parse price (aceita vírgula como decimal)
	priceStr = strings.Replace(priceStr, ",", ".", 1)
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		http.Redirect(w, r, "/admin/produtos?error=Preço inválido", http.StatusSeeOther)
		return
	}

	var categoryID *int
	if categoryStr != "" && categoryStr != "0" {
		catID, err := strconv.Atoi(categoryStr)
		if err == nil {
			categoryID = &catID
		}
	}

	// Valida JSON de opções
	var optPtr *string
	if optionsStr != "" {
		type valObj []interface{}
		var js valObj
		if err := json.Unmarshal([]byte(optionsStr), &js); err != nil {
			http.Redirect(w, r, "/admin/produtos?error=JSON de opcionais inválido. Siga o exemplo.", http.StatusSeeOther)
			return
		}
		optPtr = &optionsStr
	}

	// Upload de múltiplas imagens
	var imagesList []string
	imageURL := ""

	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		files := r.MultipartForm.File["images"]
		for _, fileHeader := range files {
			f, err := fileHeader.Open()
			if err != nil {
				log.Printf("Erro ao abrir arquivo enviado: %v", err)
				continue
			}
			uploadedURL, err := saveUploadedFile(f, fileHeader.Filename)
			f.Close()
			if err != nil {
				log.Printf("Erro ao salvar imagem: %v", err)
				continue
			}
			imagesList = append(imagesList, uploadedURL)
		}
	}

	// Se houver imagens carregadas
	var imgPtr *string
	if len(imagesList) > 0 {
		imageURL = imagesList[0] // A primeira imagem é a de capa/principal
		imgBytes, err := json.Marshal(imagesList)
		if err == nil {
			imgStr := string(imgBytes)
			imgPtr = &imgStr
		}
	}

	product := &database.Product{
		ShopID:      shop.ID,
		CategoryID:  categoryID,
		Name:        name,
		Description: description,
		Price:       price,
		ImageURL:    imageURL,
		IsAvailable: true,
		Options:     optPtr,
		Images:      imgPtr,
	}

	if err := h.DB.CreateProduct(r.Context(), product); err != nil {
		log.Printf("Erro ao criar produto: %v", err)
		http.Redirect(w, r, "/admin/produtos?error=Erro ao criar produto", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/produtos?success=Produto criado com sucesso!", http.StatusSeeOther)
}

// HandleDeleteProduct deleta um produto (HTMX)
func (h *Handlers) HandleDeleteProduct(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Error(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := h.DB.DeleteProduct(r.Context(), id, shop.ID); err != nil {
		http.Error(w, "Erro ao deletar produto", http.StatusInternalServerError)
		return
	}

	// Retorna vazio para HTMX remover o elemento
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

// HandleToggleProduct alterna disponibilidade (HTMX)
func (h *Handlers) HandleToggleProduct(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Error(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	product, err := h.DB.ToggleProductAvailability(r.Context(), id, shop.ID)
	if err != nil {
		http.Error(w, "Erro ao atualizar produto", http.StatusInternalServerError)
		return
	}

	// Retorna o novo estado como HTML parcial para HTMX
	statusClass := "bg-emerald-500/10 text-emerald-400"
	statusText := "Disponível"
	if !product.IsAvailable {
		statusClass = "bg-red-500/10 text-red-400"
		statusText = "Indisponível"
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<span class="inline-flex items-center px-2.5 py-1 rounded-lg text-xs font-medium %s">%s</span>`, statusClass, statusText)
}

// ==================== CATEGORIES ====================

// HandleCategories lista e gerencia categorias
func (h *Handlers) HandleCategories(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
		return
	}

	categories, err := h.DB.ListCategoriesByShop(r.Context(), shop.ID)
	if err != nil {
		log.Printf("Erro ao listar categorias: %v", err)
	}

	data := map[string]interface{}{
		"User":       user,
		"Shop":       shop,
		"Categories": categories,
		"Success":    r.URL.Query().Get("success"),
		"Error":      r.URL.Query().Get("error"),
	}

	if err := h.Tmpl.Render(w, "admin", "admin/categories.html", data); err != nil {
		log.Printf("Erro ao renderizar categorias: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleCreateCategory cria uma nova categoria
func (h *Handlers) HandleCreateCategory(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Error(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	positionStr := r.FormValue("position")

	if name == "" {
		http.Redirect(w, r, "/admin/categorias?error=Nome é obrigatório", http.StatusSeeOther)
		return
	}

	position := 0
	if positionStr != "" {
		position, _ = strconv.Atoi(positionStr)
	}

	if _, err := h.DB.CreateCategory(r.Context(), shop.ID, name, position); err != nil {
		log.Printf("Erro ao criar categoria: %v", err)
		http.Redirect(w, r, "/admin/categorias?error=Erro ao criar categoria", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/categorias?success=Categoria criada!", http.StatusSeeOther)
}

// HandleDeleteCategory deleta uma categoria (HTMX)
func (h *Handlers) HandleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Error(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := h.DB.DeleteCategory(r.Context(), id, shop.ID); err != nil {
		http.Error(w, "Erro ao deletar categoria", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

// ==================== SHOP CONFIG ====================

// HandleShopConfig renderiza as configurações da loja
func (h *Handlers) HandleShopConfig(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	shop := middleware.GetShopFromContext(r)

	// Parse business hours JSON into a map for easier template rendering
	var businessHoursMap map[string]map[string]string
	if shop != nil && shop.BusinessHours != nil && *shop.BusinessHours != "" {
		if err := json.Unmarshal([]byte(*shop.BusinessHours), &businessHoursMap); err != nil {
			log.Printf("[CONFIG] Erro ao parsear business_hours: %v", err)
			businessHoursMap = nil
		}
	}

	data := map[string]interface{}{
		"User":         user,
		"Shop":         shop,
		"BusinessHours": businessHoursMap,
		"Success":      r.URL.Query().Get("success"),
		"Error":        r.URL.Query().Get("error"),
	}

	if err := h.Tmpl.Render(w, "admin", "admin/config.html", data); err != nil {
		log.Printf("Erro ao renderizar config: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleShopConfigPost salva as configurações da loja
func (h *Handlers) HandleShopConfigPost(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	shop := middleware.GetShopFromContext(r)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Redirect(w, r, "/admin/config?error=Erro ao processar formulário", http.StatusSeeOther)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	slug := strings.TrimSpace(r.FormValue("slug"))
	whatsapp := cleanWhatsAppNumber(strings.TrimSpace(r.FormValue("whatsapp")))
	primaryColor := strings.TrimSpace(r.FormValue("primary_color"))
	deliveryFeeStr := strings.TrimSpace(r.FormValue("delivery_fee"))

	if name == "" || slug == "" || whatsapp == "" {
		http.Redirect(w, r, "/admin/config?error=Preencha os campos obrigatórios", http.StatusSeeOther)
		return
	}

	if primaryColor == "" {
		primaryColor = "#8B5CF6"
	}

	// Taxa de entrega
	deliveryFeeStr = strings.Replace(deliveryFeeStr, ",", ".", 1)
	deliveryFee, _ := strconv.ParseFloat(deliveryFeeStr, 64)

	// Horários de funcionamento (constrói o JSON)
	days := []string{"mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	hoursMap := make(map[string]map[string]string)
	for _, day := range days {
		open := strings.TrimSpace(r.FormValue("hours_" + day + "_open"))
		close := strings.TrimSpace(r.FormValue("hours_" + day + "_close"))
		if open != "" && close != "" {
			hoursMap[day] = map[string]string{
				"open":  open,
				"close": close,
			}
		}
	}
	var bhPtr *string
	if len(hoursMap) > 0 {
		bhBytes, _ := json.Marshal(hoursMap)
		bhStr := string(bhBytes)
		bhPtr = &bhStr
	}
	log.Printf("[CONFIG] Hours Map parsed from form: %+v", hoursMap)
	if bhPtr != nil {
		log.Printf("[CONFIG] BusinessHours JSON string: %s", *bhPtr)
	} else {
		log.Printf("[CONFIG] BusinessHours JSON is nil")
	}

	// Upload de logo
	logoURL := ""
	if shop != nil {
		logoURL = shop.LogoURL
	}
	logoFile, logoHeader, err := r.FormFile("logo")
	if err == nil {
		defer logoFile.Close()
		logoURL, err = saveUploadedFile(logoFile, logoHeader.Filename)
		if err != nil {
			log.Printf("Erro ao salvar logo: %v", err)
		}
	}

	// Upload de banner (capa)
	bannerURL := ""
	if shop != nil {
		bannerURL = shop.BannerURL
	}
	bannerFile, bannerHeader, err := r.FormFile("banner")
	if err == nil {
		defer bannerFile.Close()
		bannerURL, err = saveUploadedFile(bannerFile, bannerHeader.Filename)
		if err != nil {
			log.Printf("Erro ao salvar banner: %v", err)
		}
	}

	if shop != nil {
		// Atualiza loja existente
		shop.Name = name
		shop.Slug = generateSlug(slug)
		shop.WhatsappNumber = whatsapp
		shop.PrimaryColor = primaryColor
		shop.LogoURL = logoURL
		shop.BannerURL = bannerURL
		shop.DeliveryFee = deliveryFee
		shop.BusinessHours = bhPtr

		if err := h.DB.UpdateShop(r.Context(), shop); err != nil {
			log.Printf("Erro ao atualizar loja: %v", err)
			http.Redirect(w, r, "/admin/config?error=Erro ao salvar", http.StatusSeeOther)
			return
		}
	} else {
		// Cria nova loja
		newShop := &database.Shop{
			UserID:         user.ID,
			Name:           name,
			Slug:           generateSlug(slug),
			WhatsappNumber: whatsapp,
			PrimaryColor:   primaryColor,
			LogoURL:        logoURL,
			BannerURL:      bannerURL,
			DeliveryFee:    deliveryFee,
			BusinessHours:  bhPtr,
		}
		if err := h.DB.CreateShop(r.Context(), newShop); err != nil {
			log.Printf("Erro ao criar loja: %v", err)
			http.Redirect(w, r, "/admin/config?error=Erro ao criar loja", http.StatusSeeOther)
			return
		}
	}

	http.Redirect(w, r, "/admin/config?success=Configurações salvas!", http.StatusSeeOther)
}

// ==================== UPLOAD ====================

// saveUploadedFile salva um arquivo enviado no diretório de uploads
func saveUploadedFile(file io.Reader, filename string) (string, error) {
	// Cria diretório de uploads se não existir
	uploadDir := filepath.Join("public", "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return "", fmt.Errorf("erro ao criar diretório de uploads: %w", err)
	}

	// Gera nome único
	ext := filepath.Ext(filename)
	newFilename := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	filePath := filepath.Join(uploadDir, newFilename)

	dst, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("erro ao criar arquivo: %w", err)
	}
	defer dst.Close()

	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("erro ao copiar arquivo: %w", err)
	}

	// Retorna URL relativa
	return "/static/uploads/" + newFilename, nil
}

// generateSlug gera um slug URL-friendly a partir de um texto
func generateSlug(text string) string {
	slug := strings.ToLower(strings.TrimSpace(text))
	
	// Substituições de caracteres acentuados
	replacements := map[string]string{
		"á": "a", "à": "a", "ã": "a", "â": "a", "ä": "a",
		"é": "e", "è": "e", "ê": "e", "ë": "e",
		"í": "i", "ì": "i", "î": "i", "ï": "i",
		"ó": "o", "ò": "o", "õ": "o", "ô": "o", "ö": "o",
		"ú": "u", "ù": "u", "û": "u", "ü": "u",
		"ç": "c", "ñ": "n",
	}
	for old, new := range replacements {
		slug = strings.ReplaceAll(slug, old, new)
	}
	
	// Remove caracteres não alfanuméricos
	var result strings.Builder
	for _, ch := range slug {
		if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == ' ' {
			result.WriteRune(ch)
		}
	}
	slug = result.String()
	
	// Substitui espaços por hífens
	slug = strings.ReplaceAll(slug, " ", "-")
	
	// Remove hífens duplicados
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	
	// Remove hífens do início e fim
	slug = strings.Trim(slug, "-")
	
	return slug
}

// ==================== ADMIN ORDERS ====================

// HandleOrders lista todos os pedidos recebidos pela loja
func (h *Handlers) HandleOrders(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
		return
	}

	orders, err := h.DB.ListOrdersByShop(r.Context(), shop.ID)
	if err != nil {
		log.Printf("Erro ao listar pedidos: %v", err)
	}

	data := map[string]interface{}{
		"User":   user,
		"Shop":   shop,
		"Orders": orders,
	}

	if err := h.Tmpl.Render(w, "admin", "admin/orders.html", data); err != nil {
		log.Printf("Erro ao renderizar pedidos: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleOrderStatusPost altera o status de um pedido via HTMX
func (h *Handlers) HandleOrderStatusPost(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Error(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	status := strings.TrimSpace(r.FormValue("status"))
	validStatus := map[string]bool{
		"Pendente":   true,
		"Preparando": true,
		"Enviado":    true,
		"Concluido":  true,
		"Cancelado":  true,
	}

	if !validStatus[status] {
		http.Error(w, "Status inválido", http.StatusBadRequest)
		return
	}

	if err := h.DB.UpdateOrderStatus(r.Context(), id, shop.ID, status); err != nil {
		log.Printf("Erro ao atualizar status: %v", err)
		http.Error(w, "Erro ao atualizar status", http.StatusInternalServerError)
		return
	}

	// Retorna o novo HTML do badge status com cores correspondentes
	var badgeClass string
	switch status {
	case "Pendente":
		badgeClass = "bg-amber-500/10 text-amber-400 border-amber-500/20"
	case "Preparando":
		badgeClass = "bg-blue-500/10 text-blue-400 border-blue-500/20"
	case "Enviado":
		badgeClass = "bg-violet-500/10 text-violet-400 border-violet-500/20"
	case "Concluido":
		badgeClass = "bg-emerald-500/10 text-emerald-400 border-emerald-500/20"
	case "Cancelado":
		badgeClass = "bg-red-500/10 text-red-400 border-red-500/20"
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<span class="inline-flex items-center px-2.5 py-1 rounded-full text-xs font-bold border %s">%s</span>`, badgeClass, status)
}

// ==================== COUPONS ADMIN ====================

// HandleCoupons lista todos os cupons
func (h *Handlers) HandleCoupons(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
		return
	}

	coupons, err := h.DB.ListCouponsByShop(r.Context(), shop.ID)
	if err != nil {
		log.Printf("Erro ao listar cupons: %v", err)
	}

	data := map[string]interface{}{
		"User":    user,
		"Shop":    shop,
		"Coupons": coupons,
		"Success": r.URL.Query().Get("success"),
		"Error":   r.URL.Query().Get("error"),
	}

	if err := h.Tmpl.Render(w, "admin", "admin/coupons.html", data); err != nil {
		log.Printf("Erro ao renderizar cupons: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleCreateCoupon cria um cupom para a loja
func (h *Handlers) HandleCreateCoupon(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Error(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	code := strings.ToUpper(strings.TrimSpace(r.FormValue("code")))
	couponType := strings.TrimSpace(r.FormValue("type"))
	valueStr := strings.TrimSpace(r.FormValue("value"))

	if code == "" || couponType == "" || valueStr == "" {
		http.Redirect(w, r, "/admin/cupons?error=Preencha todos os campos obrigatórios", http.StatusSeeOther)
		return
	}

	if couponType != "percentage" && couponType != "fixed" {
		http.Redirect(w, r, "/admin/cupons?error=Tipo de cupom inválido", http.StatusSeeOther)
		return
	}

	valueStr = strings.Replace(valueStr, ",", ".", 1)
	value, err := strconv.ParseFloat(valueStr, 64)
	if err != nil || value <= 0 {
		http.Redirect(w, r, "/admin/cupons?error=Valor de desconto inválido", http.StatusSeeOther)
		return
	}

	coupon := &database.Coupon{
		ShopID:   shop.ID,
		Code:     code,
		Type:     couponType,
		Value:    value,
		IsActive: true,
	}

	if err := h.DB.CreateCoupon(r.Context(), coupon); err != nil {
		log.Printf("Erro ao criar cupom: %v", err)
		http.Redirect(w, r, "/admin/cupons?error=Este código de cupom já existe", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/cupons?success=Cupom criado com sucesso!", http.StatusSeeOther)
}

// HandleDeleteCoupon remove um cupom (HTMX)
func (h *Handlers) HandleDeleteCoupon(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Error(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := h.DB.DeleteCoupon(r.Context(), id, shop.ID); err != nil {
		log.Printf("Erro ao deletar cupom: %v", err)
		http.Error(w, "Erro ao deletar cupom", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}

// cleanWhatsAppNumber limpa caracteres não-numéricos e adiciona o DDI +55 (Brasil) se ausente
func cleanWhatsAppNumber(number string) string {
	var clean strings.Builder
	for _, r := range number {
		if r >= '0' && r <= '9' {
			clean.WriteRune(r)
		}
	}
	s := clean.String()
	if s == "" {
		return ""
	}
	// Se tiver 11 dígitos ou menos, com certeza não tem o DDI do Brasil (55),
	// mesmo que os primeiros dígitos sejam 55 (que seria o DDD 55 do RS).
	if len(s) <= 11 {
		s = "55" + s
	} else if !strings.HasPrefix(s, "55") {
		// Se tiver mais de 11 dígitos mas não começar com 55, adicionamos
		s = "55" + s
	}
	return s
}


