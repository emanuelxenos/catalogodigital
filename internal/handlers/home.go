package handlers

import (
	"log"
	"net/http"

	"catalogo/internal/database"
)

// HandleHome renderiza a landing page do SaaS
func (h *Handlers) HandleHome(w http.ResponseWriter, r *http.Request) {
	// Carrega todos os planos cadastrados para exibir na Landing Page
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
		log.Printf("Erro ao carregar planos na home: %v", err)
	}

	data := map[string]interface{}{
		"Title": "Cataloger | Seu catálogo online via WhatsApp",
		"Plans": plans,
	}

	if err := h.Tmpl.Render(w, "base", "home.html", data); err != nil {
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}

// HandleTerms renderiza a página pública de Termos e Condições de Uso
func (h *Handlers) HandleTerms(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Termos e Condições de Uso | Cataloger",
	}

	if err := h.Tmpl.Render(w, "base", "termos.html", data); err != nil {
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}
