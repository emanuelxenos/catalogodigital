package handlers

import (
	"net/http"
)

// HandleHome renderiza a landing page do SaaS
func (h *Handlers) HandleHome(w http.ResponseWriter, r *http.Request) {
	data := map[string]interface{}{
		"Title": "Catálogo Digital | Seu catálogo online via WhatsApp",
	}

	if err := h.Tmpl.Render(w, "base", "home.html", data); err != nil {
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}
