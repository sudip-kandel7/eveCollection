package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/sudip-kandel7/eveCollection/internal/algorithms"
	"github.com/sudip-kandel7/eveCollection/internal/db"
	"github.com/sudip-kandel7/eveCollection/internal/validator"
	"github.com/sudip-kandel7/eveCollection/views"
	"github.com/sudip-kandel7/eveCollection/views/components"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type SignupForm struct {
	Name                string `form:"name"`
	Phone               string `form:"phone"`
	Email               string `form:"email"`
	Address             string `form:"address"`
	Password            string `form:"password"`
	ConfirmPassword     string `form:"confirm_password"`
	validator.Validator `form:"-"`
}

type LoginForm struct {
	Identifier          string `form:"identifier"`
	Password            string `form:"password"`
	validator.Validator `form:"-"`
}

type authUser struct {
	ID           int32
	Salt         string
	Role         string
	PasswordHash string
}

type profileForm struct {
	Name                string `form:"name"`
	Email               string `form:"email"`
	Phone               string `form:"phone"`
	Address             string `form:"address"`
	validator.Validator `form:"-"`
}

type VariantForm struct {
	Color string `form:"color"`
	Size  string `form:"size"`
	Stock string `form:"stock"`
	Price string `form:"price"`
}

type ProductForm struct {
	Name                string        `form:"name"`
	CategoryID          string        `form:"category_id"`
	Description         string        `form:"description"`
	BasePrice           string        `form:"base_price"`
	IsActive            string        `form:"is_active"`
	Tags                []string      `form:"tags[]"`
	ExistingImages      []string      `form:"existing_images[]"`
	Variants            []VariantForm `form:"variants"`
	validator.Validator `form:"-"`
}

type parsedVariant struct {
	size  string
	color string
	stock int32
	price pgtype.Numeric
}

type parsedProduct struct {
	categoryID int32
	basePrice  float64
	tags       []string
	variants   []parsedVariant
}

const (
	maxImageBytes    = 2 << 20
	maxProductImages = 7
	maxProductTags   = 10
	maxProductName   = 100
	maxDescription   = 2000
	maxMoney         = 50000.0
	maxVariants      = 50
	maxReviewText    = 1000
	relatedLimit     = 4
)

type reviewForm struct {
	ProductID           string `form:"product_id"`
	Rating              string `form:"rating"`
	ReviewText          string `form:"review_text"`
	validator.Validator `form:"-"`
}

type cartForm struct {
	VariantID string `form:"variant_id"`
	Quantity  string `form:"quantity"`
}

func validatePhone(form *validator.Validator, phone string) {
	form.CheckField(validator.NotBlank(phone), "phone", "This field cannot be blank")
	form.CheckField(validator.Matches(phone, validator.PhoneRX), "phone", "Phone must start with 97 or 98 and contain only digits")
	form.CheckField(validator.MinChars(phone, 10), "phone", "Phone number must be 10 digits")
	form.CheckField(validator.MaxChars(phone, 10), "phone", "Phone number must be 10 digits")
	if len(phone) == 10 {
		form.CheckField(validator.NotRepeatingDigits(phone[2:]), "phone", "Phone number looks invalid")
	}
}

func validateAddress(form *validator.Validator, address string) {
	form.CheckField(validator.NotBlank(address), "address", "Address cannot be blank")
	form.CheckField(validator.MaxChars(address, 200), "address", "Address cannot exceed 200 characters")
	form.CheckField(validator.MinChars(address, 5), "address", "Address looks too short")
	form.CheckField(validator.Matches(address, validator.AddressRX), "address", "Address contains invalid characters")
}

func (app *Application) Home(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	categories, err := app.queries.ListCategoriesWithProducts(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	productsCard, err := app.queries.ListProductsCard(r.Context(), db.ListProductsCardParams{
		Category: 0,
		Terms:    []string{},
		Sort:     "Featured",
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	productStats, err := app.queries.ListProductStats(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	ranked := algorithms.TopTrending(productStats, 5)

	cardByID := make(map[int32]db.ListProductsCardRow, len(productsCard))
	for _, card := range productsCard {
		cardByID[card.ID] = card
	}

	trendingCards := make([]db.ListProductsCardRow, 0, len(ranked))
	for _, product := range ranked {
		if card, ok := cardByID[product.ProductID]; ok {
			trendingCards = append(trendingCards, card)
		}
	}

	component := views.IndexRenderer(user, categories, trendingCards, productsCard)

	err = component.Render(r.Context(), w)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *Application) ProductDetails(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	images, err := app.queries.ListProductImages(r.Context(), int32(id))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	productDetails, err := app.queries.GetProductDetail(r.Context(), int32(id))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	if err := app.queries.RecordProductView(r.Context(), int32(id)); err != nil {
		app.logger.Error("failed to record product view", "product_id", id, "error", err)
	}

	productReviews, err := app.queries.ListProductReviews(r.Context(), int32(id))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	variants, err := app.queries.ListProductVariants(r.Context(), int32(id))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	categoryProductIDs, err := app.queries.GetCategoryProductIDs(r.Context(), int32(id))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	productTagIDs, err := app.queries.GetProductTagIDs(r.Context(), int32(id))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	tagProductIDs, err := app.queries.GetProductIDsByTagIDs(r.Context(), productTagIDs)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	seen := make(map[int32]bool, len(categoryProductIDs)+len(tagProductIDs))
	productIDs := make([]int32, 0, len(categoryProductIDs)+len(tagProductIDs))

	for _, pid := range categoryProductIDs {
		if pid == int32(id) || seen[pid] {
			continue
		}
		seen[pid] = true
		productIDs = append(productIDs, pid)
	}
	for _, pid := range tagProductIDs {
		if pid == int32(id) || seen[pid] {
			continue
		}
		seen[pid] = true
		productIDs = append(productIDs, pid)
	}

	recommendationFeatures, err := app.queries.GetRecommendationFeatures(r.Context(), productIDs)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	user := app.contextGetUser(r)

	recommendations := algorithms.RecommendationPoints(recommendationFeatures, productTagIDs, productDetails.CategoryID, relatedLimit)

	allCards, err := app.queries.ListProductsCard(r.Context(), db.ListProductsCardParams{
		Category: 0,
		Terms:    []string{},
		Sort:     "Featured",
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	cardByID := make(map[int32]db.ListProductsCardRow, len(allCards))
	for _, card := range allCards {
		cardByID[card.ID] = card
	}

	recommendedCards := make([]db.ListProductsCardRow, 0, len(recommendations))
	for _, recommendation := range recommendations {
		if card, ok := cardByID[recommendation.ProductID]; ok {
			recommendedCards = append(recommendedCards, card)
		}
	}
	if len(recommendedCards) == 0 {
		recommendedCards = allCards
		if len(recommendedCards) > relatedLimit {
			recommendedCards = recommendedCards[:relatedLimit]
		}
	}

	canReview := false
	if user != nil {
		canReview, err = app.queries.HasDeliveredProduct(r.Context(), db.HasDeliveredProductParams{
			CustomerID: user.ID,
			ProductID:  int32(id),
		})
		if err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	component := views.ProductDetailRenderer(user, productDetails, images, productReviews, variants, recommendedCards, canReview)

	err = component.Render(r.Context(), w)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *Application) AddReview(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var form reviewForm
	if err := app.decodePostForm(r, &form); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	productID, err := strconv.ParseInt(strings.TrimSpace(form.ProductID), 10, 32)
	if err != nil || productID <= 0 {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	productDetails, err := app.queries.GetProductDetail(r.Context(), int32(productID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			app.clientError(w, http.StatusNotFound)
			return
		}
		app.serverError(w, r, err)
		return
	}

	form.Rating = strings.TrimSpace(form.Rating)
	form.ReviewText = strings.TrimSpace(form.ReviewText)

	rating, ratingErr := strconv.Atoi(form.Rating)
	if ratingErr != nil {
		rating = 0
	}

	form.CheckField(rating >= 1 && rating <= 5, "rating", "Please choose a rating between 1 and 5")
	form.CheckField(validator.MaxChars(form.ReviewText, maxReviewText), "review_text",
		fmt.Sprintf("Review cannot exceed %d characters", maxReviewText))

	delivered, err := app.queries.HasDeliveredProduct(r.Context(), db.HasDeliveredProductParams{
		CustomerID: user.ID,
		ProductID:  int32(productID),
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	form.CheckField(delivered, "order", "You can review this product only after your order is delivered.")

	if !form.Valid() {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.ReviewErr(form.FieldErrors), datastar.WithSelectorID("err-review"), datastar.WithModeOuter())
		sse.PatchSignals([]byte(`{"reviewSubmitting": false}`))
		return
	}

	_, err = app.queries.CreateProductReview(r.Context(), db.CreateProductReviewParams{
		ProductID:  int32(productID),
		CustomerID: user.ID,
		Rating:     int32(rating),
		ReviewText: pgtype.Text{String: form.ReviewText, Valid: form.ReviewText != ""},
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	productDetails, err = app.queries.GetProductDetail(r.Context(), int32(productID))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	productReviews, err := app.queries.ListProductReviews(r.Context(), int32(productID))
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(views.RatingSummary(productDetails), datastar.WithSelectorID("rating-summary"), datastar.WithModeOuter())
	sse.PatchElementTempl(views.ReviewSection(user, productDetails, productReviews, true), datastar.WithSelectorID("review-section"), datastar.WithModeOuter())
	sse.PatchSignals([]byte(`{"reviewRating": 0, "reviewHover": 0, "reviewText": "", "reviewSubmitting": false}`))
}

func (app *Application) AddToCart(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var form cartForm
	if err := app.decodePostForm(r, &form); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	variantID, variantErr := strconv.ParseInt(strings.TrimSpace(form.VariantID), 10, 32)
	quantity, quantityErr := strconv.Atoi(strings.TrimSpace(form.Quantity))
	if variantErr != nil || variantID <= 0 || quantityErr != nil || quantity <= 0 {
		sse := datastar.NewSSE(w, r)
		sse.PatchSignals([]byte(`{"cartSubmitting": false, "cartMessage": "Please choose a valid variant and quantity.", "cartError": true}`))
		return
	}

	row, err := app.queries.AddToCart(r.Context(), db.AddToCartParams{
		CustomerID: user.ID,
		VariantID:  int32(variantID),
		Quantity:   int32(quantity),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			sse := datastar.NewSSE(w, r)
			sse.PatchSignals([]byte(`{"cartSubmitting": false, "cartMessage": "This variant is sold out or unavailable.", "cartError": true}`))
			return
		}
		app.serverError(w, r, err)
		return
	}

	if err := app.queries.RecordProductCart(r.Context(), db.RecordProductCartParams{
		ProductID: row.ProductID,
		Quantity:  int32(quantity),
	}); err != nil {
		app.logger.Error("failed to record product cart", "product_id", row.ProductID, "error", err)
	}

	message := fmt.Sprintf("Added to cart - %d in cart", row.Quantity)
	if row.Quantity >= row.Stock {
		message = fmt.Sprintf("Cart is at max available (%d)", row.Stock)
	}

	signals, err := json.Marshal(map[string]any{
		"quantity":       1,
		"cartSubmitting": false,
		"cartMessage":    message,
		"cartError":      false,
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchSignals(signals)
}

func (app *Application) ProductsFilter(w http.ResponseWriter, r *http.Request) {

	fmt.Println("got request")

	var s struct {
		Cat    string `json:"cat"`
		Sort   string `json:"sort"`
		Search string `json:"search"`
	}

	if err := datastar.ReadSignals(r, &s); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	catID, _ := strconv.Atoi(s.Cat)

	terms := []string{}
	if search := strings.ToLower(strings.TrimSpace(s.Search)); search != "" {
		terms = strings.Fields(search)
	}

	productCard, err := app.queries.ListProductsCard(r.Context(), db.ListProductsCardParams{
		Category: int32(catID),
		Terms:    terms,
		Sort:     s.Sort,
	})

	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(views.ProductGrid(productCard))

}

func (app *Application) getCartItems(r *http.Request, userID int32) ([]db.GetCartItemsRow, error) {
	return app.queries.GetCartItems(r.Context(), userID)
}

func (app *Application) Profile(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	cartItems, err := app.getCartItems(r, user.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	tab := r.URL.Query().Get("tab")
	switch tab {
	case "profile", "cart", "orders", "messages", "reviews":
	default:
		tab = "profile"
	}

	var orders []views.OrderView
	if tab == "orders" {
		orders, err = app.loadOrderViews(r, user.ID)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	views.ProfileRenderer(user, tab, cartItems, orders).Render(r.Context(), w)
}

func (app *Application) About(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)
	views.AboutRenderer(user).Render(r.Context(), w)
}

func (app *Application) Orders(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/profile?tab=orders", http.StatusSeeOther)
}

func (app *Application) OrderSuccess(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	orderID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || orderID <= 0 {
		app.clientError(w, http.StatusNotFound)
		return
	}

	order, err := app.queries.GetOrderByCustomer(r.Context(), db.GetOrderByCustomerParams{
		ID:         int32(orderID),
		CustomerID: user.ID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			app.clientError(w, http.StatusNotFound)
			return
		}
		app.serverError(w, r, err)
		return
	}

	views.OrderSuccessRenderer(user, order).Render(r.Context(), w)
}

func validOrderStatus(status string) bool {
	switch status {
	case "placed", "packed", "out_for_delivery", "delivered":
		return true
	}
	return false
}

func orderViewFromOrder(order db.GetOrderRow, items []db.ListOrderItemsRow) views.OrderView {
	return views.OrderView{
		ID:               order.ID,
		CustomerName:     order.CustomerName,
		CustomerPhone:    order.CustomerPhone,
		Status:           order.Status,
		PaymentMethod:    order.PaymentMethod,
		PaymentStatus:    order.PaymentStatus,
		TotalAmount:      order.TotalAmount,
		TotalQuantity:    order.TotalQuantity,
		DeliveryAddress:  order.DeliveryAddress,
		DeliveryDistrict: order.DeliveryDistrict,
		DeliveryPhone:    order.DeliveryPhone,
		CreatedAt:        order.CreatedAt,
		Items:            items,
	}
}

func (app *Application) loadOrderDetail(w http.ResponseWriter, r *http.Request) (db.GetOrderRow, []db.ListOrderItemsRow, bool) {
	orderID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || orderID <= 0 {
		app.clientError(w, http.StatusNotFound)
		return db.GetOrderRow{}, nil, false
	}

	order, err := app.queries.GetOrder(r.Context(), int32(orderID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			app.clientError(w, http.StatusNotFound)
			return db.GetOrderRow{}, nil, false
		}
		app.serverError(w, r, err)
		return db.GetOrderRow{}, nil, false
	}

	items, err := app.queries.ListOrderItems(r.Context(), []int32{int32(orderID)})
	if err != nil {
		app.serverError(w, r, err)
		return db.GetOrderRow{}, nil, false
	}

	return order, items, true
}

func (app *Application) UpdateOrderStatus(w http.ResponseWriter, r *http.Request) {
	var form struct {
		Status string `form:"status"`
	}
	if err := app.decodePostForm(r, &form); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	orderID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || orderID <= 0 {
		app.clientError(w, http.StatusNotFound)
		return
	}

	if !validOrderStatus(form.Status) {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	affected, err := app.queries.UpdateOrderStatus(r.Context(), db.UpdateOrderStatusParams{
		ID:     int32(orderID),
		Status: form.Status,
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if affected == 0 {
		app.clientError(w, http.StatusNotFound)
		return
	}

	orders, err := app.loadOrderViews(r, 0)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(views.OrdersAdminContent(orders), datastar.WithSelectorID("orders-admin"))
}

func (app *Application) UpdateOrderPaymentStatus(w http.ResponseWriter, r *http.Request) {
	var form struct {
		PaymentStatus string `form:"payment_status"`
	}
	if err := app.decodePostForm(r, &form); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	orderID, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || orderID <= 0 {
		app.clientError(w, http.StatusNotFound)
		return
	}

	if form.PaymentStatus != "pending" && form.PaymentStatus != "paid" {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	order, err := app.queries.GetOrder(r.Context(), int32(orderID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			app.clientError(w, http.StatusNotFound)
			return
		}
		app.serverError(w, r, err)
		return
	}
	if order.PaymentMethod != "cod" {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	affected, err := app.queries.UpdateOrderPaymentStatus(r.Context(), db.UpdateOrderPaymentStatusParams{
		ID:            int32(orderID),
		PaymentStatus: form.PaymentStatus,
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if affected == 0 {
		app.clientError(w, http.StatusNotFound)
		return
	}

	orders, err := app.loadOrderViews(r, 0)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(views.OrdersAdminContent(orders), datastar.WithSelectorID("orders-admin"))
}

func (app *Application) AdminOrderDetail(w http.ResponseWriter, r *http.Request) {
	order, items, ok := app.loadOrderDetail(w, r)
	if !ok {
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(views.OrderDetailModal(orderViewFromOrder(order, items), true), datastar.WithSelectorID("order-detail-modal"), datastar.WithModeInner())
	sse.PatchSignals([]byte(`{"orderModalOpen": true}`))
}

func (app *Application) CustomerOrderDetail(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	order, items, ok := app.loadOrderDetail(w, r)
	if !ok {
		return
	}

	if order.CustomerID != user.ID {
		app.clientError(w, http.StatusNotFound)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(views.OrderDetailModal(orderViewFromOrder(order, items), false), datastar.WithSelectorID("order-detail-modal"), datastar.WithModeInner())
	sse.PatchSignals([]byte(`{"orderModalOpen": true}`))
}

func (app *Application) Cart(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	cartItems, err := app.getCartItems(r, user.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	err = views.ProfileRenderer(user, "cart", cartItems, nil).Render(r.Context(), w)
	if err != nil {
		app.serverError(w, r, err)
	}
}

func (app *Application) Checkout(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	raw := app.sessionManager.GetString(r.Context(), "checkout")

	var intent checkoutIntent
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &intent); err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	var products []db.GetProductByVariantIDRow

	if intent.Source == "product" {
		if len(intent.Items) == 0 {
			http.Redirect(w, r, "/product", http.StatusSeeOther)
			return
		}

		item := intent.Items[0]
		product, err := app.queries.GetProductByVariantID(r.Context(), db.GetProductByVariantIDParams{
			Quantity:  item.Quantity,
			VariantID: item.VariantID,
		})
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		products = append(products, product)

	} else if intent.Source == "cart" {
		if len(intent.Items) == 0 {
			http.Redirect(w, r, "/cart", http.StatusSeeOther)
			return
		}

		for _, item := range intent.Items {
			product, err := app.queries.GetProductByVariantID(r.Context(), db.GetProductByVariantIDParams{
				Quantity:  item.Quantity,
				VariantID: item.VariantID,
			})
			if err != nil {
				app.serverError(w, r, err)
				return
			}
			products = append(products, product)
		}

	}

	if err := views.CheckoutRenderer(user, products, intent.Source).Render(r.Context(), w); err != nil {
		app.serverError(w, r, err)
	}
}

type checkoutIntent struct {
	Source string               `json:"source"`
	Items  []checkoutIntentItem `json:"items"`
}

type checkoutIntentItem struct {
	VariantID int32 `json:"variant_id"`
	Quantity  int32 `json:"quantity"`
}

type cartCheckoutForm struct {
	Items string `form:"items"`
}

type buynowForm struct {
	VariantId string `form:"variant_id"`
	Quantity  string `form:"quantity"`
}

type placeOrderForm struct {
	Name                string `form:"name"`
	Phone               string `form:"phone"`
	Address             string `form:"address"`
	District            string `form:"district"`
	Payment             string `form:"payment"`
	validator.Validator `form:"-"`
}

type orderItem struct {
	VariantID int32
	ProductID int32
	Quantity  int32
	Price     float64
}

func (app *Application) BuyNow(w http.ResponseWriter, r *http.Request) {
	var form buynowForm
	if err := app.decodePostForm(r, &form); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(form.VariantId)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	quantity, err := strconv.Atoi(form.Quantity)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	intent := checkoutIntent{
		Source: "product",
		Items: []checkoutIntentItem{{
			VariantID: int32(id),
			Quantity:  int32(quantity),
		}},
	}

	data, err := json.Marshal(intent)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.sessionManager.Put(r.Context(), "checkout", string(data))
	http.Redirect(w, r, "/checkout", http.StatusSeeOther)
}

func (app *Application) cartCheckout(w http.ResponseWriter, r *http.Request) {

	var form cartCheckoutForm
	if err := app.decodePostForm(r, &form); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	var items []checkoutIntentItem
	if err := json.Unmarshal([]byte(form.Items), &items); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	intent := checkoutIntent{
		Source: "cart",
		Items:  items,
	}

	data, err := json.Marshal(intent)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.sessionManager.Put(r.Context(), "checkout", string(data))
	http.Redirect(w, r, "/checkout", http.StatusSeeOther)
}

func (app *Application) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)
	if user == nil {
		app.clientError(w, http.StatusForbidden)
		return
	}

	raw := app.sessionManager.GetString(r.Context(), "checkout")

	var intent checkoutIntent
	if raw != "" {
		if err := json.Unmarshal([]byte(raw), &intent); err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	if len(intent.Items) == 0 {
		target := "/"
		if intent.Source == "cart" {
			target = "/cart"
		}
		sse := datastar.NewSSE(w, r)
		if err := sse.Redirect(target); err != nil {
			app.serverError(w, r, err)
		}
		return
	}

	var form placeOrderForm
	if err := app.decodePostForm(r, &form); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	validatePhone(&form.Validator, form.Phone)
	validateAddress(&form.Validator, form.Address)

	if !form.Valid() {
		sse := datastar.NewSSE(w, r)
		_ = sse.PatchSignals([]byte(`{"checkoutSubmitting": false, "orderError": ""}`))
		sse.PatchElementTempl(components.CheckoutErrors(form.FieldErrors))
		return
	}

	if form.Payment != "cod" && form.Payment != "esewa" {
		app.patchCheckoutError(w, r, "Please choose a valid payment method.")
		return
	}

	total := 0.0
	if form.District == "Kathmandu" {
		total = 150
	} else {
		total = 250
	}

	items := make([]orderItem, 0, len(intent.Items))
	for _, item := range intent.Items {
		info, err := app.queries.GetVariantForOrder(r.Context(), item.VariantID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				app.patchCheckoutError(w, r, "One of your items is no longer available.")
				return
			}
			app.serverError(w, r, err)
			return
		}

		if info.Stock < item.Quantity {
			app.patchCheckoutError(w, r, "One of your items does not have enough stock.")
			return
		}

		total += info.Price * float64(item.Quantity)
		items = append(items, orderItem{
			VariantID: item.VariantID,
			ProductID: info.ProductID,
			Quantity:  item.Quantity,
			Price:     info.Price,
		})
	}

	totalNumeric, err := numericFromFloat(total)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	orderInfo := db.CreateOrderParams{
		CustomerID:       user.ID,
		Status:           "placed",
		PaymentMethod:    form.Payment,
		TotalAmount:      totalNumeric,
		DeliveryAddress:  form.Address,
		DeliveryDistrict: form.District,
		DeliveryPhone:    form.Phone,
	}

	if form.Payment == "esewa" {
		app.patchCheckoutError(w, r, "eSewa payment is not available yet. Please choose cash on delivery.")
		return
	}

	orderId, err := app.queries.CreateOrder(r.Context(), orderInfo)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	cleanupOrder := func() {
		if err := app.queries.DeleteOrder(r.Context(), orderId); err != nil {
			app.logger.Error("failed to clean up order", "order_id", orderId, "error", err)
		}
	}

	for _, item := range items {
		unitPrice, err := numericFromFloat(item.Price)
		if err != nil {
			cleanupOrder()
			app.serverError(w, r, err)
			return
		}

		err = app.queries.CreateOrderItem(r.Context(), db.CreateOrderItemParams{
			OrderID:   orderId,
			VariantID: item.VariantID,
			Quantity:  item.Quantity,
			UnitPrice: unitPrice,
		})
		if err != nil {
			cleanupOrder()
			app.logger.Error("failed to create order item", "order_id", orderId, "error", err)
			app.patchCheckoutError(w, r, "We could not place your order. Please try again.")
			return
		}
	}

	decremented := make([]orderItem, 0, len(items))
	for _, item := range items {
		affected, err := app.queries.DecreaseVariantStock(r.Context(), db.DecreaseVariantStockParams{
			ID:    item.VariantID,
			Stock: item.Quantity,
		})
		if err != nil || affected == 0 {
			for _, done := range decremented {
				_ = app.queries.IncreaseVariantStock(r.Context(), db.IncreaseVariantStockParams{
					ID:    done.VariantID,
					Stock: done.Quantity,
				})
			}
			cleanupOrder()
			if err != nil {
				app.logger.Error("failed to decrease stock", "variant_id", item.VariantID, "error", err)
			}
			app.patchCheckoutError(w, r, "One of your items just went out of stock. Please try again.")
			return
		}
		decremented = append(decremented, item)
	}

	if intent.Source == "cart" {
		for _, item := range items {
			affected, err := app.queries.SubtractOrderedCartQuantity(r.Context(), db.SubtractOrderedCartQuantityParams{
				CustomerID: user.ID,
				VariantID:  item.VariantID,
				Quantity:   item.Quantity,
			})
			if err != nil {
				app.logger.Error("failed to update cart item", "variant_id", item.VariantID, "error", err)
				continue
			}
			if affected == 0 {
				if err := app.queries.RemoveCartItem(r.Context(), db.RemoveCartItemParams{
					CustomerID: user.ID,
					VariantID:  item.VariantID,
				}); err != nil {
					app.logger.Error("failed to remove cart item", "variant_id", item.VariantID, "error", err)
				}
			}
		}
	}

	for _, item := range items {
		if err := app.queries.RecordProductPurchase(r.Context(), db.RecordProductPurchaseParams{
			ProductID: item.ProductID,
			Purchases: item.Quantity,
		}); err != nil {
			app.logger.Error("failed to record product purchase", "product_id", item.ProductID, "error", err)
		}
	}

	app.sessionManager.Remove(r.Context(), "checkout")

	sse := datastar.NewSSE(w, r)
	if err := sse.Redirect(fmt.Sprintf("/orders/%d/success", orderId)); err != nil {
		app.serverError(w, r, err)
	}
}

func (app *Application) patchCheckoutError(w http.ResponseWriter, r *http.Request, message string) {
	sse := datastar.NewSSE(w, r)
	signals, err := json.Marshal(map[string]any{
		"orderError":         message,
		"checkoutSubmitting": false,
	})
	if err != nil {
		return
	}
	_ = sse.PatchSignals(signals)
}

func (app *Application) RemoveFromCart(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	var form cartForm
	if err := app.decodePostForm(r, &form); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	variantID, variantErr := strconv.ParseInt(strings.TrimSpace(form.VariantID), 10, 32)
	if variantErr != nil || variantID <= 0 {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	err := app.queries.RemoveCartItem(r.Context(), db.RemoveCartItemParams{
		CustomerID: user.ID,
		VariantID:  int32(variantID),
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	cartItems, err := app.getCartItems(r, user.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchSignals([]byte(views.CartSignals(cartItems)))
	sse.PatchElementTempl(views.CartItems(cartItems),
		datastar.WithSelectorID("cart-items"), datastar.WithModeOuter())
}

func (app *Application) Reviews(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	cartItems, err := app.getCartItems(r, user.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	views.ProfileRenderer(user, "reviews", cartItems, nil).Render(r.Context(), w)
}

func (app *Application) Messages(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)

	cartItems, err := app.getCartItems(r, user.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	views.ProfileRenderer(user, "messages", cartItems, nil).Render(r.Context(), w)
}

func (app *Application) Login(w http.ResponseWriter, r *http.Request) {
	views.LoginRenderer(views.LoginFormData{}).Render(r.Context(), w)
}

func (app *Application) Register(w http.ResponseWriter, r *http.Request) {
	views.RegisterRenderer(views.RegisterFormData{}).Render(r.Context(), w)
}

func (app *Application) AdminLogin(w http.ResponseWriter, r *http.Request) {
	views.AdminLoginRenderer(views.AdminLoginFormData{}).Render(r.Context(), w)
}

func (app *Application) Dashboard(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)
	categories, err := app.queries.ListCategoriesWithPC(r.Context())

	if err != nil {
		app.serverError(w, r, err)
		return
	}

	products, err := app.queries.ListProducts(r.Context(), db.ListProductsParams{})
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	orders, err := app.loadOrderViews(r, 0)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	views.DashboardRenderer(user, "products", categories, products, orders).Render(r.Context(), w)
}

func (app *Application) loadOrderViews(r *http.Request, customerID int32) ([]views.OrderView, error) {
	orders, err := app.queries.ListOrders(r.Context(), customerID)
	if err != nil {
		return nil, err
	}

	if len(orders) == 0 {
		return []views.OrderView{}, nil
	}

	orderIDs := make([]int32, 0, len(orders))
	for _, order := range orders {
		orderIDs = append(orderIDs, order.ID)
	}

	items, err := app.queries.ListOrderItems(r.Context(), orderIDs)
	if err != nil {
		return nil, err
	}

	itemsByOrder := make(map[int32][]db.ListOrderItemsRow, len(orders))
	for _, item := range items {
		itemsByOrder[item.OrderID] = append(itemsByOrder[item.OrderID], item)
	}

	orderViews := make([]views.OrderView, 0, len(orders))
	for _, order := range orders {
		orderViews = append(orderViews, views.OrderView{
			ID:               order.ID,
			CustomerName:     order.CustomerName,
			CustomerPhone:    order.CustomerPhone,
			Status:           order.Status,
			PaymentMethod:    order.PaymentMethod,
			PaymentStatus:    order.PaymentStatus,
			TotalAmount:      order.TotalAmount,
			TotalQuantity:    order.TotalQuantity,
			DeliveryAddress:  order.DeliveryAddress,
			DeliveryDistrict: order.DeliveryDistrict,
			DeliveryPhone:    order.DeliveryPhone,
			CreatedAt:        order.CreatedAt,
			Items:            itemsByOrder[order.ID],
		})
	}

	return orderViews, nil
}

func (app *Application) SearchProducts(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(r.URL.Query().Get("q"))

	var categoryID int32
	if raw := strings.TrimSpace(r.URL.Query().Get("category")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			categoryID = int32(parsed)
		}
	}

	products, err := app.queries.ListProducts(r.Context(), db.ListProductsParams{
		Search:   search,
		Category: categoryID,
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(views.ProductCount(len(products)))
	sse.PatchElementTempl(views.ProductsList(products))
}

func (app *Application) Logout(w http.ResponseWriter, r *http.Request) {
	err := app.sessionManager.Destroy(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *Application) Signup(w http.ResponseWriter, r *http.Request) {
	var form SignupForm

	err := app.decodePostForm(r, &form)

	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(validator.NotBlank(form.Name), "name", "This field cannot be blank")
	form.CheckField(validator.MinChars(form.Name, 3), "name", "This field must be at least 3 characters long")
	form.CheckField(validator.MaxChars(form.Name, 27), "name", "This field must not be more than 27 characters")
	form.CheckField(validator.Matches(form.Name, validator.NameRX), "name", "Name can only contain letters and spaces")

	emailTaken, err := app.queries.EmailTaken(r.Context(), form.Email)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	form.CheckField(!emailTaken, "email", "Email already in use")

	form.CheckField(validator.NotBlank(form.Email), "email", "This field cannot be blank")
	form.CheckField(validator.Matches(form.Email, validator.EmailRX), "email", "Please enter valid email address")

	phoneTaken, err := app.queries.PhoneTaken(r.Context(), form.Phone)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	form.CheckField(!phoneTaken, "phone", "Phone number already in use")
	validatePhone(&form.Validator, form.Phone)
	validateAddress(&form.Validator, form.Address)

	form.CheckField(validator.NotBlank(form.Password), "password", "This field cannot be blank")
	form.CheckField(validator.MinChars(form.Password, 8), "password", "Password must be at least 8 characters")
	form.CheckField(validator.MaxChars(form.Password, 17), "password", "Password cannot exceed 17 characters")
	form.CheckField(validator.Matches(form.Password, validator.UpperRX), "password", "Password must contain at least one uppercase letter")
	form.CheckField(validator.Matches(form.Password, validator.LowerRX), "password", "Password must contain at least one lowercase letter")
	form.CheckField(validator.Matches(form.Password, validator.DigitRX), "password", "Password must contain at least one number")
	form.CheckField(validator.Matches(form.Password, validator.SymbolRX), "password", "Password must contain at least one symbol")

	form.CheckField(validator.NotBlank(form.ConfirmPassword), "confirm_password", "This field cannot be blank")
	form.CheckField(form.Password == form.ConfirmPassword, "confirm_password", "Passwords do not match")

	if !form.Valid() {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.RegisterErrors(form.FieldErrors, form.NonFieldErrors))
		return
	}

	hashedPassword, salt, err := algorithms.HashPassword(form.Password)

	if err != nil {
		app.serverError(w, r, err)
		return
	}

	row, err := app.queries.CreateUser(r.Context(), db.CreateUserParams{
		Name:         form.Name,
		Phone:        form.Phone,
		Email:        form.Email,
		Address:      form.Address,
		PasswordHash: hashedPassword,
		Salt:         salt,
	})

	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.logger.Info("user logged in", "userID", row.ID)

	app.sessionManager.Put(r.Context(), "authenticatedUserID", row.ID)

	savedID := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	app.logger.Info("session user ID", "userID", savedID)

	if err := app.commitSession(w, r); err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	if err := sse.Redirect("/"); err != nil {
		app.serverError(w, r, err)
	}

}

func (app *Application) Signin(w http.ResponseWriter, r *http.Request) {
	var form LoginForm

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(validator.NotBlank(form.Identifier), "identifier", "This field cannot be blank")
	form.CheckField(validator.NotBlank(form.Password), "password", "This field cannot be blank")

	var isEmail, isPhone bool
	if form.Identifier != "" {
		isEmail = validator.Matches(form.Identifier, validator.EmailRX)
		isPhone = validator.Matches(form.Identifier, validator.PhoneRX) && len(form.Identifier) == 10

		if !isEmail && !isPhone {
			form.AddFieldError("identifier", "Enter a valid email or phone number")
		}
	}

	if !form.Valid() {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.LoginErrors(form.FieldErrors, form.NonFieldErrors))
		return
	}

	var user authUser
	var dbErr error

	if isEmail {
		row, e := app.queries.GetUserByEmail(r.Context(), form.Identifier)
		dbErr = e
		user = authUser{ID: row.ID, Salt: row.Salt, Role: row.Role, PasswordHash: row.PasswordHash}
	} else {
		row, e := app.queries.GetUserByPhone(r.Context(), form.Identifier)
		dbErr = e
		user = authUser{ID: row.ID, Salt: row.Salt, Role: row.Role, PasswordHash: row.PasswordHash}
	}

	if dbErr != nil {
		form.AddNonFieldError("Invalid credentials")
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.LoginErrors(form.FieldErrors, form.NonFieldErrors))
		return
	}

	valid := algorithms.VerifyPassword(form.Password, user.Salt, user.PasswordHash)

	if !valid {
		form.AddNonFieldError("Invalid credentials")
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.LoginErrors(form.FieldErrors, form.NonFieldErrors))
		return
	}

	if user.Role != "customer" {
		form.AddNonFieldError("Invalid credentials")
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.LoginErrors(form.FieldErrors, form.NonFieldErrors))
		return
	}

	err = app.sessionManager.RenewToken(r.Context())

	if err != nil {
		app.serverError(w, r, err)
		return
	}

	app.logger.Info("user logged in", "userID", user.ID)

	app.sessionManager.Put(r.Context(), "authenticatedUserID", user.ID)

	savedID := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	app.logger.Info("session user ID", "userID", savedID)

	if err := app.commitSession(w, r); err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	if err := sse.Redirect("/"); err != nil {
		app.serverError(w, r, err)
	}
}

func (app *Application) AdminSignin(w http.ResponseWriter, r *http.Request) {
	attempts := app.sessionManager.GetInt(r.Context(), "adminLoginAttempts")
	if attempts >= 2 {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var form LoginForm

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	fail := func() {
		attempts++
		app.sessionManager.Put(r.Context(), "adminLoginAttempts", attempts)

		if attempts >= 2 {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		data := views.AdminLoginFormData{
			Identifier: form.Identifier,
			ErrorMsg:   "Invalid credentials",
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
		views.AdminLoginRenderer(data).Render(r.Context(), w)
	}

	if !validator.NotBlank(form.Identifier) || !validator.NotBlank(form.Password) {
		fail()
		return
	}

	isEmail := validator.Matches(form.Identifier, validator.EmailRX)
	isPhone := validator.Matches(form.Identifier, validator.PhoneRX) && len(form.Identifier) == 10
	if !isEmail && !isPhone {
		fail()
		return
	}

	var user authUser
	var dbErr error

	if isEmail {
		row, e := app.queries.GetUserByEmail(r.Context(), form.Identifier)
		dbErr = e
		user = authUser{ID: row.ID, Salt: row.Salt, PasswordHash: row.PasswordHash}
	} else {
		row, e := app.queries.GetUserByPhone(r.Context(), form.Identifier)
		dbErr = e
		user = authUser{ID: row.ID, Salt: row.Salt, PasswordHash: row.PasswordHash}
	}

	if dbErr != nil {
		fail()
		return
	}

	valid := algorithms.VerifyPassword(form.Password, user.Salt, user.PasswordHash)
	if !valid {
		fail()
		return
	}

	role, err := app.queries.GetUserRole(r.Context(), user.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	if role != "admin" {
		fail()
		return
	}

	err = app.sessionManager.RenewToken(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	app.sessionManager.Remove(r.Context(), "adminLoginAttempts")
	app.sessionManager.Put(r.Context(), "authenticatedUserID", user.ID)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (app *Application) updateProfile(w http.ResponseWriter, r *http.Request) {

	user := app.contextGetUser(r)
	if user == nil {
		app.clientError(w, http.StatusForbidden)
		return
	}

	var form profileForm

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(validator.NotBlank(form.Name), "name", "This field cannot be blank")
	form.CheckField(validator.MinChars(form.Name, 3), "name", "This field must be at least 3 characters long")
	form.CheckField(validator.MaxChars(form.Name, 27), "name", "This field must not be more than 27 characters")
	form.CheckField(validator.Matches(form.Name, validator.NameRX), "name", "Name can only contain letters and spaces")

	if form.Email != user.Email {
		emailTaken, err := app.queries.EmailTaken(r.Context(), form.Email)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		form.CheckField(!emailTaken, "email", "Email already in use")
	}

	form.CheckField(validator.NotBlank(form.Email), "email", "This field cannot be blank")
	form.CheckField(validator.Matches(form.Email, validator.EmailRX), "email", "Please enter valid email address")

	if form.Phone != user.Phone {
		phoneTaken, err := app.queries.PhoneTaken(r.Context(), form.Phone)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		form.CheckField(!phoneTaken, "phone", "Phone number already in use")
	}

	validatePhone(&form.Validator, form.Phone)
	validateAddress(&form.Validator, form.Address)

	if !form.Valid() {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.UpdateErrors(form.FieldErrors))
		return
	}

	updated, err := app.queries.UpdateUser(r.Context(), db.UpdateUserParams{
		ID:      user.ID,
		Name:    form.Name,
		Email:   form.Email,
		Phone:   form.Phone,
		Address: form.Address,
	})

	if err != nil {
		app.serverError(w, r, err)
		return
	}

	freshUser := &db.User{
		ID:        updated.ID,
		Name:      updated.Name,
		Email:     updated.Email,
		Phone:     updated.Phone,
		Address:   updated.Address,
		Role:      updated.Role,
		CreatedAt: updated.CreatedAt,
		UpdatedAt: updated.UpdatedAt,
	}

	cartItems, err := app.getCartItems(r, freshUser.ID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(views.ProfileRenderer(freshUser, "profile", cartItems, nil))
	sse.PatchSignals([]byte(`{"updating": false}`))
}

type categoryForm struct {
	Name                string `form:"name"`
	IsActive            string `form:"is_active"`
	validator.Validator `form:"-"`
}

func (app *Application) Categories(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = maxImageBytes

	var form categoryForm

	if err := r.ParseMultipartForm(maxUploadSize); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		app.serverError(w, r, err)
		return
	}

	err := app.decodePostForm(r, &form)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(validator.NotBlank(form.Name), "name", "This field cannot be blank")
	form.CheckField(validator.MinChars(form.Name, 2), "name", "Name must be at least 2 characters")
	form.CheckField(validator.MaxChars(form.Name, 50), "name", "Name cannot exceed 50 characters")
	form.CheckField(validator.Matches(form.Name, validator.CategoryNameRX), "name", "Name contains invalid characters")

	if form.Name != "" {
		taken, err := app.queries.CategoryNameTaken(r.Context(), form.Name)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		form.CheckField(!taken, "name", "A category with this name already exists")
	}

	var imageBytes []byte

	file, header, err := r.FormFile("image")
	switch {
	case err == nil:
		defer file.Close()

		if header.Size > maxUploadSize {
			form.AddFieldError("image", "Image must be 2MB or less")
		}

		buf := make([]byte, 512)
		if _, err := io.ReadFull(file, buf); err != nil &&
			!errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			app.serverError(w, r, err)
			return
		}

		if contentType := http.DetectContentType(buf); contentType != "image/jpeg" && contentType != "image/png" {
			form.AddFieldError("image", "Image must be JPG, JPEG or PNG")
		}

		if form.Valid() {
			file.Seek(0, io.SeekStart)
			imageBytes, err = processImage(file)
			if err != nil {
				app.serverError(w, r, err)
				return
			}
		}
	case errors.Is(err, http.ErrMissingFile), errors.Is(err, http.ErrNotMultipart):
		form.AddFieldError("image", "Please upload an image")
	default:
		app.serverError(w, r, err)
		return
	}

	if !form.Valid() {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.CategoryErrors(form.FieldErrors))
		sse.PatchSignals([]byte(`{"submitting": false}`))
		return
	}

	uploadDir := "uploads"

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		app.serverError(w, r, err)
		return
	}

	filename := fmt.Sprintf("%d_%s.jpg", time.Now().UnixNano(), slugify(form.Name))
	savePath := filepath.Join(uploadDir, filename)

	if err := os.WriteFile(savePath, imageBytes, 0644); err != nil {
		app.serverError(w, r, err)
		return
	}

	imageURL := "/uploads/" + filename

	isActive := form.IsActive == "on"

	_, err = app.queries.CreateCategory(r.Context(), db.CreateCategoryParams{
		Name:     form.Name,
		ImageUrl: pgtype.Text{String: imageURL, Valid: true},
		IsActive: isActive,
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	categories, err := app.queries.ListCategoriesWithPC(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(components.CategoryErrors(map[string]string{}))
	sse.PatchElementTempl(views.CategoryAdminContent(categories), datastar.WithSelectorID("category-admin"))
	sse.PatchElementTempl(views.ProductCategorySelect(categories), datastar.WithSelectorID("product-category-select"), datastar.WithModeOuter())
	sse.PatchSignals([]byte(`{"submitting": false, "catname": "", "catimage": "", "catpreview": ""}`))
}

func processImage(file multipart.File) ([]byte, error) {
	img, err := imaging.Decode(file)
	if err != nil {
		return nil, err
	}

	resized := imaging.Resize(img, 600, 0, imaging.Lanczos)

	var buf bytes.Buffer
	err = imaging.Encode(&buf, resized, imaging.JPEG, imaging.JPEGQuality(80))
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func slugify(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "-")
	return s
}

func textOrEmpty(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}

func (app *Application) UpdateCategory(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = maxImageBytes

	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	categoryID := int32(id)

	category, err := app.queries.GetCategory(r.Context(), categoryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.clientError(w, http.StatusNotFound)
			return
		}

		app.serverError(w, r, err)
		return
	}

	oldImageURL := textOrEmpty(category.ImageUrl)

	var form categoryForm

	if err := r.ParseMultipartForm(maxUploadSize); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		app.serverError(w, r, err)
		return
	}

	if err := app.decodePostForm(r, &form); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	form.CheckField(validator.NotBlank(form.Name), "name", "This field cannot be blank")
	form.CheckField(validator.MinChars(form.Name, 2), "name", "Name must be at least 2 characters")
	form.CheckField(validator.MaxChars(form.Name, 50), "name", "Name cannot exceed 50 characters")
	form.CheckField(validator.Matches(form.Name, validator.CategoryNameRX), "name", "Name contains invalid characters")

	if form.Valid() && form.Name != category.Name {
		taken, err := app.queries.CategoryNameTakenForUpdate(
			r.Context(),
			db.CategoryNameTakenForUpdateParams{
				Name: form.Name,
				ID:   categoryID,
			},
		)
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		form.CheckField(!taken, "name", "A category with this name already exists")
	}

	if !form.Valid() {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.CategoryErrors(form.FieldErrors))
		sse.PatchSignals([]byte(`{"submitting": false}`))
		return
	}

	var imageURL string
	var hasNewImage bool

	file, header, err := r.FormFile("image")

	switch {
	case err == nil:
		hasNewImage = true
		defer file.Close()

		if header.Size > maxUploadSize {
			form.AddFieldError("image", "Image must be 2MB or less")
		}

		buf := make([]byte, 512)

		if _, err := io.ReadFull(file, buf); err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			app.serverError(w, r, err)
			return
		}

		contentType := http.DetectContentType(buf)

		if contentType != "image/jpeg" && contentType != "image/png" {
			form.AddFieldError("image", "Image must be JPG, JPEG or PNG")
		}

		if form.Valid() {
			if _, err := file.Seek(0, io.SeekStart); err != nil {
				app.serverError(w, r, err)
				return
			}

			imageBytes, err := processImage(file)
			if err != nil {
				app.serverError(w, r, err)
				return
			}

			uploadDir := "uploads"

			if err := os.MkdirAll(uploadDir, 0755); err != nil {
				app.serverError(w, r, err)
				return
			}

			filename := fmt.Sprintf("%d_%s.jpg", time.Now().UnixNano(), slugify(form.Name))
			savePath := filepath.Join(uploadDir, filename)

			if err := os.WriteFile(savePath, imageBytes, 0644); err != nil {
				app.serverError(w, r, err)
				return
			}

			imageURL = "/uploads/" + filename
		}

	case errors.Is(err, http.ErrMissingFile), errors.Is(err, http.ErrNotMultipart):

	default:
		app.serverError(w, r, err)
		return
	}

	if !form.Valid() {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.CategoryErrors(form.FieldErrors))
		sse.PatchSignals([]byte(`{"submitting": false}`))
		return
	}

	isActive := form.IsActive == "on"

	if hasNewImage {
		_, err = app.queries.UpdateCategoryWithImage(
			r.Context(),
			db.UpdateCategoryWithImageParams{
				ID:   categoryID,
				Name: form.Name,
				ImageUrl: pgtype.Text{
					String: imageURL,
					Valid:  true,
				},
				IsActive: isActive,
			},
		)
	} else {
		_, err = app.queries.UpdateCategory(
			r.Context(),
			db.UpdateCategoryParams{
				ID:       categoryID,
				Name:     form.Name,
				IsActive: isActive,
			},
		)
	}

	if err != nil {
		if hasNewImage {
			_ = os.Remove(filepath.Join("uploads", filepath.Base(imageURL)))
		}

		app.serverError(w, r, err)
		return
	}

	if hasNewImage && oldImageURL != "" {
		oldImagePath := filepath.Join("uploads", filepath.Base(oldImageURL))

		if err := os.Remove(oldImagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			app.serverError(w, r, err)
			return
		}
	}

	categories, err := app.queries.ListCategoriesWithPC(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(components.CategoryErrors(map[string]string{}))
	sse.PatchElementTempl(views.CategoryAdminContent(categories), datastar.WithSelectorID("category-admin"))
	sse.PatchElementTempl(views.ProductCategorySelect(categories), datastar.WithSelectorID("product-category-select"), datastar.WithModeOuter())
	sse.PatchSignals([]byte(`{"submitting": false, "formmode": "create", "categoryid": "0", "catname": "", "catimage": "", "catpreview": "", "catactive": true}`))
}

func (app *Application) DeleteCategory(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	categoryID := int32(id)

	category, err := app.queries.GetCategory(r.Context(), categoryID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			app.clientError(w, http.StatusNotFound)
			return
		}

		app.serverError(w, r, err)
		return
	}

	imageURL := textOrEmpty(category.ImageUrl)

	if err := app.queries.DeleteCategory(r.Context(), categoryID); err != nil {
		app.serverError(w, r, err)
		return
	}

	if imageURL != "" {
		imagePath := filepath.Join("uploads", filepath.Base(imageURL))

		if err := os.Remove(imagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			app.serverError(w, r, err)
			return
		}
	}

	categories, err := app.queries.ListCategoriesWithPC(r.Context())
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(views.CategoryAdminContent(categories), datastar.WithSelectorID("category-admin"))
	sse.PatchElementTempl(views.ProductCategorySelect(categories), datastar.WithSelectorID("product-category-select"), datastar.WithModeOuter())
	sse.PatchSignals([]byte(`{"formmode":"create","categoryid":"0","catname":"","catimage":"","catpreview":"","catactive":true,"submitting":false}`))
}

func validateProductImages(r *http.Request, form *ProductForm) []*multipart.FileHeader {
	if r.MultipartForm == nil {
		return nil
	}

	headers := r.MultipartForm.File["images"]
	if len(headers) == 0 {
		return nil
	}

	images := make([]*multipart.FileHeader, 0, len(headers))

	for _, header := range headers {
		if header.Size > maxImageBytes {
			form.AddFieldError("images", "Each image must be 2MB or less")
			return nil
		}

		file, err := header.Open()
		if err != nil {
			form.AddFieldError("images", "Could not read the uploaded image")
			return nil
		}

		buf := make([]byte, 512)
		n, err := io.ReadFull(file, buf)
		file.Close()

		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			form.AddFieldError("images", "Could not read the uploaded image")
			return nil
		}

		if contentType := http.DetectContentType(buf[:n]); contentType != "image/jpeg" && contentType != "image/png" {
			form.AddFieldError("images", "Images must be JPG, JPEG or PNG")
			return nil
		}

		images = append(images, header)
	}

	return images
}

func numericFromFloat(value float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	err := n.Scan(strconv.FormatFloat(value, 'f', 2, 64))
	return n, err
}

func (app *Application) validateProduct(r *http.Request, form *ProductForm, productID int32, isUpdate bool) (parsedProduct, error) {
	var out parsedProduct

	// name
	form.Name = strings.TrimSpace(form.Name)
	form.CheckField(validator.NotBlank(form.Name), "name", "This field cannot be blank")
	form.CheckField(validator.MinChars(form.Name, 2), "name", "Name must be at least 2 characters")
	form.CheckField(validator.MaxChars(form.Name, maxProductName), "name",
		fmt.Sprintf("Name cannot exceed %d characters", maxProductName))
	form.CheckField(validator.Matches(form.Name, validator.ProductNameRX), "name",
		"Name can only contain letters, numbers, spaces and . ' & -")

	if validator.NotBlank(form.Name) {
		var taken bool
		var err error
		if isUpdate {
			taken, err = app.queries.ProductNameTakenForUpdate(r.Context(), db.ProductNameTakenForUpdateParams{
				Lower: form.Name,
				ID:    productID,
			})
		} else {
			taken, err = app.queries.ProductNameTaken(r.Context(), form.Name)
		}
		if err != nil {
			return out, err
		}
		form.CheckField(!taken, "name", "A product with this name already exists")
	}

	// description
	form.Description = strings.TrimSpace(form.Description)
	form.CheckField(validator.MaxChars(form.Description, maxDescription), "description",
		fmt.Sprintf("Description cannot exceed %d characters", maxDescription))

	// category
	if !validator.NotBlank(form.CategoryID) {
		form.AddFieldError("category", "Please choose a category")
	} else if parsed, err := strconv.ParseInt(strings.TrimSpace(form.CategoryID), 10, 32); err != nil {
		form.AddFieldError("category", "Please choose a category")
	} else {
		out.categoryID = int32(parsed)
		if _, err := app.queries.GetCategory(r.Context(), out.categoryID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				form.AddFieldError("category", "That category does not exist")
			} else {
				return out, err
			}
		}
	}

	// base price
	if !validator.NotBlank(form.BasePrice) {
		form.AddFieldError("price", "This field cannot be blank")
	} else if parsed, err := strconv.ParseFloat(strings.TrimSpace(form.BasePrice), 64); err != nil {
		form.AddFieldError("price", "Base price must be a number")
	} else if !validator.PositiveFloat(parsed) {
		form.AddFieldError("price", "Base price must be greater than 0")
	} else if !validator.MaxFloat(parsed, maxMoney) {
		form.AddFieldError("price", "Base price cannot exceed Rs. 50,000")
	} else {
		out.basePrice = parsed
	}

	// tags
	seenTags := make(map[string]bool, len(form.Tags))
	tags := make([]string, 0, len(form.Tags))
	for _, raw := range form.Tags {
		tag := strings.TrimSpace(raw)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if seenTags[key] {
			continue
		}
		seenTags[key] = true
		tags = append(tags, tag)
	}
	out.tags = tags

	form.CheckField(len(tags) > 0, "tags", "Please add at least one tag")
	form.CheckField(len(tags) <= maxProductTags, "tags", fmt.Sprintf("You can add at most %d tags", maxProductTags))

	for _, tag := range tags {
		valid := validator.Matches(tag, validator.TagRX) &&
			validator.MinChars(tag, 2) &&
			validator.MaxChars(tag, 30)
		form.CheckField(valid, "tags", "Tags must be 2-30 characters using letters, numbers, spaces or hyphens")
	}

	// variants
	form.CheckField(len(form.Variants) > 0, "variants", "Please add at least one variant")
	form.CheckField(len(form.Variants) <= maxVariants, "variants",
		fmt.Sprintf("You can add at most %d variants", maxVariants))

	variants := make([]parsedVariant, 0, len(form.Variants))
	pairs := make([][]string, 0, len(form.Variants))

	for _, v := range form.Variants {
		color := strings.TrimSpace(v.Color)
		size := strings.TrimSpace(v.Size)

		form.CheckField(validator.NotBlank(color) &&
			validator.Matches(color, validator.VariantRX) &&
			validator.MaxChars(color, 50), "variants",
			"Each variant needs a color")

		form.CheckField(validator.NotBlank(size) &&
			validator.Matches(size, validator.VariantRX) &&
			validator.MaxChars(size, 20), "variants",
			"Each variant needs a size")

		stock := int32(0)
		if parsed, err := strconv.Atoi(strings.TrimSpace(v.Stock)); err != nil {
			form.AddFieldError("variants", "Stock must be a whole number")
		} else if parsed < 0 {
			form.AddFieldError("variants", "Stock cannot be negative")
		} else {
			stock = int32(parsed)
		}

		price := pgtype.Numeric{}
		if raw := strings.TrimSpace(v.Price); raw != "" {
			parsed, err := strconv.ParseFloat(raw, 64)
			switch {
			case err != nil:
				form.AddFieldError("variants", "Variant price must be a number")
			case !validator.NonNegativeFloat(parsed):
				form.AddFieldError("variants", "Variant price cannot be negative")
			case !validator.MaxFloat(parsed, maxMoney):
				form.AddFieldError("variants", "Variant price cannot exceed Rs. 50,000")
			default:
				numeric, err := numericFromFloat(parsed)
				if err != nil {
					form.AddFieldError("variants", "Variant price must be a number")
				} else {
					price = numeric
				}
			}
		}

		pairs = append(pairs, []string{size, color})
		variants = append(variants, parsedVariant{size: size, color: color, stock: stock, price: price})
	}
	out.variants = variants

	form.CheckField(!validator.HasDuplicatePair(pairs), "variants",
		"Two variants share the same size and color")

	return out, nil
}

func (app *Application) Products(w http.ResponseWriter, r *http.Request) {
	var form ProductForm

	if err := r.ParseMultipartForm(maxImageBytes); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		app.serverError(w, r, err)
		return
	}

	if err := app.decodePostForm(r, &form); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	parsed, err := app.validateProduct(r, &form, 0, false)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	// images
	images := validateProductImages(r, &form)
	if len(images) == 0 {
		form.AddFieldError("images", "Please upload at least one image")
	}
	if len(images) > maxProductImages {
		form.AddFieldError("images", fmt.Sprintf("You can upload at most %d images", maxProductImages))
	}

	if !form.Valid() {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.ProductErrors(form.FieldErrors))
		sse.PatchSignals([]byte(`{"psubmitting": false}`))
		return
	}

	basePriceNumeric, err := numericFromFloat(parsed.basePrice)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	product, err := app.queries.CreateProduct(r.Context(), db.CreateProductParams{
		Name:        form.Name,
		Description: pgtype.Text{String: form.Description, Valid: form.Description != ""},
		CategoryID:  parsed.categoryID,
		BasePrice:   basePriceNumeric,
		IsActive:    form.IsActive == "on",
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	for _, v := range parsed.variants {
		err := app.queries.CreateProductVariant(r.Context(), db.CreateProductVariantParams{
			ProductID:     product.ID,
			Size:          pgtype.Text{String: v.size, Valid: true},
			Color:         pgtype.Text{String: v.color, Valid: true},
			Stock:         v.stock,
			PriceOverride: v.price,
		})
		if err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	for _, tag := range parsed.tags {
		tagID, err := app.queries.UpsertTag(r.Context(), tag)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		if err := app.queries.CreateProductTag(r.Context(), db.CreateProductTagParams{
			ProductID: product.ID,
			TagID:     tagID,
		}); err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	slug := slugify(form.Name)

	for i, header := range images {
		file, err := header.Open()
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			file.Close()
			app.serverError(w, r, err)
			return
		}

		imageBytes, err := processImage(file)
		file.Close()
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		filename := fmt.Sprintf("%d_%s_%d.jpg", time.Now().UnixNano(), slug, i)
		if err := os.WriteFile(filepath.Join("uploads", filename), imageBytes, 0644); err != nil {
			app.serverError(w, r, err)
			return
		}
		if err := app.queries.CreateProductImage(r.Context(), db.CreateProductImageParams{
			ProductID: product.ID,
			ImageUrl:  "/uploads/" + filename,
			SortOrder: int32(i),
		}); err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	products, err := app.queries.ListProducts(r.Context(), db.ListProductsParams{})
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(components.ProductErrors(map[string]string{}))
	sse.PatchElementTempl(views.ProductCount(len(products)))
	sse.PatchElementTempl(views.ProductsList(products))
	sse.PatchSignals([]byte(`{"psubmitting": false, "pname": "", "pdescription": "", "pprice": "", "pcategory": "", "pvisible": true}`))
	sse.ExecuteScript("resetProductExtras()")
}

func (app *Application) UpdateProduct(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}
	productID := int32(id)

	if err := r.ParseMultipartForm(maxImageBytes); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		app.serverError(w, r, err)
		return
	}

	if _, err := app.queries.GetProduct(r.Context(), productID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			app.clientError(w, http.StatusNotFound)
			return
		}
		app.serverError(w, r, err)
		return
	}

	var form ProductForm
	if err := app.decodePostForm(r, &form); err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}

	parsed, err := app.validateProduct(r, &form, productID, true)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	newImages := validateProductImages(r, &form)

	keptImages := make([]string, 0, len(form.ExistingImages))
	kept := make(map[string]bool, len(form.ExistingImages))
	for _, url := range form.ExistingImages {
		url = strings.TrimSpace(url)
		if url == "" || kept[url] {
			continue
		}
		kept[url] = true
		keptImages = append(keptImages, url)
	}

	app.logger.Debug("update product", "product_id", id, "kept_images", keptImages, "new_images", len(newImages))

	if len(keptImages)+len(newImages) == 0 {
		form.AddFieldError("images", "Please upload at least one image")
	}
	if len(keptImages)+len(newImages) > maxProductImages {
		form.AddFieldError("images", fmt.Sprintf("You can upload at most %d images", maxProductImages))
	}

	if !form.Valid() {
		sse := datastar.NewSSE(w, r)
		sse.PatchElementTempl(components.ProductErrors(form.FieldErrors))
		sse.PatchSignals([]byte(`{"psubmitting": false}`))
		return
	}

	basePriceNumeric, err := numericFromFloat(parsed.basePrice)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	ctx := r.Context()

	tx, err := app.pool.Begin(ctx)
	if err != nil {
		app.serverError(w, r, err)
		return
	}
	defer tx.Rollback(ctx)

	qtx := app.queries.WithTx(tx)

	if _, err := qtx.UpdateProduct(ctx, db.UpdateProductParams{
		ID:          productID,
		Name:        form.Name,
		Description: pgtype.Text{String: form.Description, Valid: form.Description != ""},
		CategoryID:  parsed.categoryID,
		BasePrice:   basePriceNumeric,
		IsActive:    form.IsActive == "on",
	}); err != nil {
		app.serverError(w, r, err)
		return
	}

	// variants: upsert the submitted (size,color) pairs, delete the ones removed
	existingVariants, err := qtx.ListProductVariants(ctx, productID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	submitted := make(map[string]bool, len(parsed.variants))
	for _, v := range parsed.variants {
		submitted[v.size+"\x00"+v.color] = true
	}

	for _, v := range parsed.variants {
		if err := qtx.UpsertProductVariant(ctx, db.UpsertProductVariantParams{
			ProductID:     productID,
			Size:          pgtype.Text{String: v.size, Valid: true},
			Color:         pgtype.Text{String: v.color, Valid: true},
			Stock:         v.stock,
			PriceOverride: v.price,
		}); err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	for _, ev := range existingVariants {
		if submitted[textOrEmpty(ev.Size)+"\x00"+textOrEmpty(ev.Color)] {
			continue
		}
		if err := qtx.DeleteProductVariant(ctx, db.DeleteProductVariantParams{
			ProductID: productID,
			Size:      ev.Size,
			Color:     ev.Color,
		}); err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	// tags: replace the product's links (the tags table itself is shared)
	if err := qtx.DeleteProductTags(ctx, productID); err != nil {
		app.serverError(w, r, err)
		return
	}
	for _, tag := range parsed.tags {
		tagID, err := qtx.UpsertTag(ctx, tag)
		if err != nil {
			app.serverError(w, r, err)
			return
		}
		if err := qtx.CreateProductTag(ctx, db.CreateProductTagParams{
			ProductID: productID,
			TagID:     tagID,
		}); err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	existingImages, err := qtx.ListProductImages(ctx, productID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	for _, url := range existingImages {
		if kept[url] {
			continue
		}
		if err := qtx.DeleteProductImage(ctx, db.DeleteProductImageParams{
			ProductID: productID,
			ImageUrl:  url,
		}); err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	slug := slugify(form.Name)
	nextSort := int32(len(existingImages))
	written := make([]string, 0, len(newImages))

	for i, header := range newImages {
		file, err := header.Open()
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		if _, err := file.Seek(0, io.SeekStart); err != nil {
			file.Close()
			app.serverError(w, r, err)
			return
		}

		imageBytes, err := processImage(file)
		file.Close()
		if err != nil {
			app.serverError(w, r, err)
			return
		}

		filename := fmt.Sprintf("%d_%s_%d.jpg", time.Now().UnixNano(), slug, i)
		if err := os.WriteFile(filepath.Join("uploads", filename), imageBytes, 0644); err != nil {
			app.serverError(w, r, err)
			return
		}
		written = append(written, filename)

		if err := qtx.CreateProductImage(ctx, db.CreateProductImageParams{
			ProductID: productID,
			ImageUrl:  "/uploads/" + filename,
			SortOrder: nextSort + int32(i),
		}); err != nil {
			app.serverError(w, r, err)
			return
		}
	}

	if err := tx.Commit(ctx); err != nil {
		for _, name := range written {
			_ = os.Remove(filepath.Join("uploads", name))
		}
		app.serverError(w, r, err)
		return
	}

	// remove the files of the dropped images now that the transaction is safe
	for _, url := range existingImages {
		if kept[url] {
			continue
		}
		_ = os.Remove(filepath.Join("uploads", filepath.Base(url)))
	}

	products, err := app.queries.ListProducts(ctx, db.ListProductsParams{})
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(components.ProductErrors(map[string]string{}))
	sse.PatchElementTempl(views.ProductCount(len(products)))
	sse.PatchElementTempl(views.ProductsList(products))
	sse.PatchSignals([]byte(`{"psubmitting": false, "pformmode": "create", "productid": "0", "pname": "", "pdescription": "", "pprice": "", "pcategory": "", "pvisible": true}`))
	sse.ExecuteScript("resetProductExtras()")
}

func (app *Application) EditProductForm(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		app.clientError(w, http.StatusBadRequest)
		return
	}
	productID := int32(id)

	product, err := app.queries.GetProduct(r.Context(), productID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	variants, err := app.queries.ListProductVariants(r.Context(), productID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	tags, err := app.queries.ListProductTags(r.Context(), productID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	images, err := app.queries.ListProductImages(r.Context(), productID)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	signals, err := json.Marshal(map[string]any{
		"pformmode":    "edit",
		"productid":    strconv.FormatInt(int64(product.ID), 10),
		"pname":        product.Name,
		"pcategory":    strconv.FormatInt(int64(product.CategoryID), 10),
		"pdescription": textOrEmpty(product.Description),
		"pprice":       strconv.FormatFloat(product.BasePrice, 'f', 2, 64),
		"pvisible":     product.IsActive,
	})
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	imageURLs, err := json.Marshal(images)
	if err != nil {
		app.serverError(w, r, err)
		return
	}

	sse := datastar.NewSSE(w, r)
	sse.PatchSignals(signals)
	sse.PatchElementTempl(views.ProductVariantRows(variants), datastar.WithSelectorID("variants-container"), datastar.WithModeInner())
	sse.PatchElementTempl(views.ProductTagChips(tags), datastar.WithSelectorID("tags"), datastar.WithModeInner())
	sse.ExecuteScript("setExistingImages(" + string(imageURLs) + "); syncVariantIndex()")
}
