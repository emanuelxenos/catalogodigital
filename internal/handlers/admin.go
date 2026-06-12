package handlers

import (
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
	whatsapp := strings.TrimSpace(r.FormValue("whatsapp"))

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

	var productCount, categoryCount int
	if shop != nil {
		productCount, _ = h.DB.CountProductsByShop(r.Context(), shop.ID)
		categoryCount, _ = h.DB.CountCategoriesByShop(r.Context(), shop.ID)
	}

	data := map[string]interface{}{
		"User":          user,
		"Shop":          shop,
		"ProductCount":  productCount,
		"CategoryCount": categoryCount,
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

	// Upload de imagem
	imageURL := ""
	file, header, err := r.FormFile("image")
	if err == nil {
		defer file.Close()
		imageURL, err = saveUploadedFile(file, header.Filename)
		if err != nil {
			log.Printf("Erro ao salvar imagem: %v", err)
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

	data := map[string]interface{}{
		"User":    user,
		"Shop":    shop,
		"Success": r.URL.Query().Get("success"),
		"Error":   r.URL.Query().Get("error"),
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
	whatsapp := strings.TrimSpace(r.FormValue("whatsapp"))
	primaryColor := strings.TrimSpace(r.FormValue("primary_color"))

	if name == "" || slug == "" || whatsapp == "" {
		http.Redirect(w, r, "/admin/config?error=Preencha os campos obrigatórios", http.StatusSeeOther)
		return
	}

	if primaryColor == "" {
		primaryColor = "#8B5CF6"
	}

	// Upload de logo
	logoURL := ""
	if shop != nil {
		logoURL = shop.LogoURL
	}
	file, header, err := r.FormFile("logo")
	if err == nil {
		defer file.Close()
		logoURL, err = saveUploadedFile(file, header.Filename)
		if err != nil {
			log.Printf("Erro ao salvar logo: %v", err)
		}
	}

	if shop != nil {
		// Atualiza loja existente
		shop.Name = name
		shop.Slug = generateSlug(slug)
		shop.WhatsappNumber = whatsapp
		shop.PrimaryColor = primaryColor
		shop.LogoURL = logoURL

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
