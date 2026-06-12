package handlers

import (
	"log"
	"net/http"
	"strconv"

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

	data := map[string]interface{}{
		"Shop":       shop,
		"Categories": categories,
		"Products":   products,
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
