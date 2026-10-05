package main

import (
	"io"
	"net/http"
	"github.com/starfederation/datastar-go/datastar"
	"github.com/sudip-kandel7/eveCollection/internal/db"
	"github.com/sudip-kandel7/eveCollection/internal/hash"
	"github.com/sudip-kandel7/eveCollection/internal/validator"
	"github.com/sudip-kandel7/eveCollection/views"
	"github.com/sudip-kandel7/eveCollection/views/components"
	"bytes"
	"fmt"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"errors"
	"time"
	"strconv"
	"github.com/disintegration/imaging"
	"github.com/jackc/pgx/v5/pgtype"
	"database/sql"
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
	Role 				 string
	PasswordHash string
}

type profileForm struct {
	Name 					string `form:"name"`
	Email 				string `form:"email"`
	Phone 				string `form:"phone"`
	Address 			string `form:"address"`
	validator.Validator  `form:"-"`
}

func (app *Application) Home(w http.ResponseWriter, r *http.Request) {
    user := app.contextGetUser(r)

    component := views.IndexRenderer(user)

    err := component.Render(r.Context(), w)
    if err != nil {
        app.serverError(w, r, err)
    }
}

func (app *Application) Profile(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)
	views.ProfileRenderer(user, "profile").Render(r.Context(), w)
}

func (app *Application) Orders(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)
	views.ProfileRenderer(user, "orders").Render(r.Context(), w)
}

func (app *Application) Reviews(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)
	views.ProfileRenderer(user, "review").Render(r.Context(), w)
}

func (app *Application) Messages(w http.ResponseWriter, r *http.Request) {
	user := app.contextGetUser(r)
	views.ProfileRenderer(user, "messages").Render(r.Context(), w)
}

func (app *Application) Login(w http.ResponseWriter, r *http.Request){
	views.LoginRenderer(views.LoginFormData{}).Render(r.Context(), w)
}

func (app *Application) Register(w http.ResponseWriter, r *http.Request){
	views.RegisterRenderer(views.RegisterFormData{}).Render(r.Context(), w)
}

func (app *Application) AdminLogin(w http.ResponseWriter, r *http.Request){
	views.AdminLoginRenderer(views.AdminLoginFormData{}).Render(r.Context(), w)
}

func (app *Application) Dashboard(w http.ResponseWriter, r *http.Request){
	user := app.contextGetUser(r)
	categories, err := app.queries.ListCategoriesWithPC(r.Context())

	if err != nil {
		app.serverError(w, r, err)
		return
	}
	
	views.DashboardRenderer(user, "products", categories).Render(r.Context(), w)
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
	form.CheckField(validator.NotBlank(form.Phone), "phone", "This field cannot be blank")
	form.CheckField(validator.Matches(form.Phone, validator.PhoneRX), "phone", "Phone must start with 97 or 98 and contain only digits")
	form.CheckField(validator.MinChars(form.Phone, 10), "phone", "Phone number must be 10 digits")
	form.CheckField(validator.MaxChars(form.Phone, 10), "phone", "Phone number must be 10 digits")
	if len(form.Phone) == 10 {
		form.CheckField(validator.NotRepeatingDigits(form.Phone[2:]), "phone", "Phone number looks invalid")
	}

	form.CheckField(validator.NotBlank(form.Address), "address", "Address cannot be blank")
	form.CheckField(validator.MaxChars(form.Address, 200), "address", "Address cannot exceed 200 characters")
  form.CheckField(validator.MinChars(form.Address, 5), "address", "Address looks too short")
 	form.CheckField(validator.Matches(form.Address, validator.AddressRX), "address", "Address contains invalid characters")
	
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

	hashedPassword, salt, err := hash.HashPassword(form.Password)

	if err != nil {
		app.serverError(w, r, err)
		return
	}

 row, err := app.queries.CreateUser(r.Context(), db.CreateUserParams{
		Name:         form.Name,
		Phone:        form.Phone,
		Email:        form.Email,
		Address: 			form.Address,
		PasswordHash: hashedPassword,
		Salt:					salt,
	})


	if err != nil {
		app.serverError(w, r, err)
		return
	}
 
	app.logger.Info("user logged in", "userID", row.ID)
	
	app.sessionManager.Put(r.Context(), "authenticatedUserID", row.ID)
	
	savedID := app.sessionManager.GetInt(r.Context(), "authenticatedUserID")
	app.logger.Info("session user ID", "userID", savedID)

 
	http.Redirect(w, r, "/", http.StatusSeeOther)
	

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

	valid := hash.VerifyPassword(form.Password, user.Salt, user.PasswordHash)

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
	
 
		http.Redirect(w, r, "/", http.StatusSeeOther)
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

	valid := hash.VerifyPassword(form.Password, user.Salt, user.PasswordHash)
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
		
		form.CheckField(validator.NotBlank(form.Phone), "phone", "This field cannot be blank")
		form.CheckField(validator.Matches(form.Phone, validator.PhoneRX), "phone", "Phone must start with 97 or 98 and contain only digits")
		form.CheckField(validator.MinChars(form.Phone, 10), "phone", "Phone number must be 10 digits")
		form.CheckField(validator.MaxChars(form.Phone, 10), "phone", "Phone number must be 10 digits")
		if len(form.Phone) == 10 {
			form.CheckField(validator.NotRepeatingDigits(form.Phone[2:]), "phone", "Phone number looks invalid")
		}
	
	form.CheckField(validator.NotBlank(form.Address), "address", "Address cannot be blank")
	form.CheckField(validator.MaxChars(form.Address, 200), "address", "Address cannot exceed 200 characters")
  form.CheckField(validator.MinChars(form.Address, 5), "address", "Address looks too short")
 	form.CheckField(validator.Matches(form.Address, validator.AddressRX), "address", "Address contains invalid characters")
	
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

	sse := datastar.NewSSE(w, r)
	sse.PatchElementTempl(views.ProfileRenderer(freshUser, "profile"))
	sse.PatchSignals([]byte(`{"updating": false}`))
}

type categoryForm struct {
	Name    string `form:"name"`
	IsActive string `form:"is_active"`
	validator.Validator `form:"-"`
}

func (app *Application) Categories(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = 5 << 20

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
			form.AddFieldError("image", "Image must be under 5MB")
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
	const maxUploadSize = 5 << 20

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
			form.AddFieldError("image", "Image must be under 5MB")
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
	sse.PatchSignals([]byte(`{"formmode":"create","categoryid":"0","catname":"","catimage":"","catpreview":"","catactive":true,"submitting":false}`))
}

type Tag struct {
	tag string
}

type productForm struct {
	Name    string `form:"name"`
	Category string `form:"category"`
	Description string `form:"description"`
	BasePrice string `form:"baseprice"`
	IsActive string `form:"is_active"`
	Tags []Tag `form:"tags"`
	validator.Validator `form:"-"`
}

func (app *Application) Products(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = 5 << 20

	var form productForm

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
			form.AddFieldError("image", "Image must be under 5MB")
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
	sse.PatchSignals([]byte(`{"submitting": false, "catname": "", "catimage": "", "catpreview": ""}`))
}
