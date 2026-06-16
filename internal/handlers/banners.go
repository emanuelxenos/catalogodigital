package handlers

import (
	"log"
	"net/http"
	"strconv"

	"catalogo/internal/database"
	"catalogo/internal/middleware"

	"github.com/go-chi/chi/v5"
)

// HandleManageBanners exibe a tela de gerenciamento de banners promocionais da loja
func (h *Handlers) HandleManageBanners(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
		return
	}

	// Trava: Banners promocionais são exclusivos do plano Diamante (ID 4)
	isLocked := shop.PlanID != 4

	var banners []database.ShopBanner
	var err error
	if !isLocked {
		banners, err = h.DB.ListShopBanners(r.Context(), shop.ID)
		if err != nil {
			log.Printf("[BANNERS] Erro ao listar banners: %v", err)
		}
	}

	data := map[string]interface{}{
		"User":     user,
		"Shop":     shop,
		"IsLocked": isLocked,
		"Banners":  banners,
		"Success":  r.URL.Query().Get("success"),
		"Error":    r.URL.Query().Get("error"),
	}

	if err := h.Tmpl.Render(w, "admin", "admin/banners.html", data); err != nil {
		log.Printf("[BANNERS] Erro ao renderizar tela de banners: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleCreateBanner processa o upload de um novo banner
func (h *Handlers) HandleCreateBanner(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Error(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	// Segurança redundante no backend
	if shop.PlanID != 4 {
		http.Redirect(w, r, "/admin/banners?error=Upgrade para plano Diamante é necessário.", http.StatusSeeOther)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Redirect(w, r, "/admin/banners?error=Erro ao ler formulário", http.StatusSeeOther)
		return
	}

	linkURL := r.FormValue("link_url")
	positionStr := r.FormValue("position")
	position, _ := strconv.Atoi(positionStr)

	file, header, err := r.FormFile("image")
	if err != nil {
		http.Redirect(w, r, "/admin/banners?error=Selecione uma imagem para o banner", http.StatusSeeOther)
		return
	}
	defer file.Close()

	// Salva arquivo físico
	imageURL, err := saveUploadedFile(file, header.Filename)
	if err != nil {
		log.Printf("[BANNERS] Erro ao salvar imagem: %v", err)
		http.Redirect(w, r, "/admin/banners?error=Erro ao salvar arquivo de imagem", http.StatusSeeOther)
		return
	}

	banner := &database.ShopBanner{
		ShopID:   shop.ID,
		ImageURL: imageURL,
		LinkURL:  linkURL,
		Position: position,
	}

	if err := h.DB.CreateShopBanner(r.Context(), banner); err != nil {
		log.Printf("[BANNERS] Erro ao inserir banner no DB: %v", err)
		http.Redirect(w, r, "/admin/banners?error=Erro ao cadastrar banner no banco de dados", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admin/banners?success=Banner cadastrado com sucesso!", http.StatusSeeOther)
}

// HandleDeleteBanner deleta um banner via HTMX
func (h *Handlers) HandleDeleteBanner(w http.ResponseWriter, r *http.Request) {
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Error(w, "Loja não configurada", http.StatusBadRequest)
		return
	}

	if shop.PlanID != 4 {
		http.Error(w, "Permissão negada", http.StatusForbidden)
		return
	}

	idStr := chi.URLParam(r, "id")
	bannerID, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "ID inválido", http.StatusBadRequest)
		return
	}

	if err := h.DB.DeleteShopBanner(r.Context(), bannerID, shop.ID); err != nil {
		log.Printf("[BANNERS] Erro ao deletar banner %d: %v", bannerID, err)
		http.Error(w, "Erro ao deletar banner", http.StatusInternalServerError)
		return
	}

	// Retorna status 200 OK para o HTMX remover a linha do banner
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(""))
}
