package handlers

import (
	"fmt"
	"log"
	"net/http"

	"catalogo/internal/middleware"
)

// HandleQRFlyer gerencia o display/panfleto de mesa com QR Code para impressao
func (h *Handlers) HandleQRFlyer(w http.ResponseWriter, r *http.Request) {
	user := middleware.GetUserFromContext(r)
	shop := middleware.GetShopFromContext(r)
	if shop == nil {
		http.Redirect(w, r, "/admin/config", http.StatusSeeOther)
		return
	}

	// Trava: Ouro (3) e Diamante (4) têm acesso a essa funcionalidade
	isLocked := shop.PlanID == 1 || shop.PlanID == 2

	// Constrói a URL pública do catálogo
	catalogURL := fmt.Sprintf("http://%s/%s", r.Host, shop.Slug)
	
	// Opcional: lê número da mesa do query param
	tableNum := r.URL.Query().Get("mesa")
	var qrDataURL string
	if tableNum != "" {
		qrDataURL = fmt.Sprintf("%s?mesa=%s", catalogURL, tableNum)
	} else {
		qrDataURL = catalogURL
	}

	// API pública de QR Code para gerar a imagem
	qrImageURL := fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=300x300&data=%s&color=0f172a&margin=10", qrDataURL)

	data := map[string]interface{}{
		"User":       user,
		"Shop":       shop,
		"IsLocked":   isLocked,
		"CatalogURL": catalogURL,
		"QRImageURL": qrImageURL,
		"TableNum":   tableNum,
	}

	// Se o lojista clicou para visualizar impressão, renderiza a página standalone limpa
	if r.URL.Query().Get("print") == "true" && !isLocked {
		if err := h.Tmpl.RenderPage(w, "admin/print_qr_flyer.html", data); err != nil {
			log.Printf("[FLYER] Erro ao renderizar display de impressao: %v", err)
			http.Error(w, "Erro interno", http.StatusInternalServerError)
		}
		return
	}

	if err := h.Tmpl.Render(w, "admin", "admin/qr_flyer.html", data); err != nil {
		log.Printf("[FLYER] Erro ao renderizar tela do flyer: %v", err)
		http.Error(w, "Erro interno", http.StatusInternalServerError)
	}
}
