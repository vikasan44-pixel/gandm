package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/google/uuid"

	"gandm/internal/auth"
	"gandm/internal/httpx"
	"gandm/internal/models"
	"gandm/internal/service"
)

// maxUploadSize caps the whole multipart body; a bit above the per-file limit
// enforced in the service layer to leave room for multipart framing overhead.
const maxUploadSize = 16 << 20 // 16 MB

type RegisterHandler struct {
	svc          *service.RegistrationService
	cookieSecure bool
}

func NewRegisterHandler(svc *service.RegistrationService, cookieSecure bool) *RegisterHandler {
	return &RegisterHandler{svc: svc, cookieSecure: cookieSecure}
}

// accessTokenBody is the token payload returned to the client. Only the
// short-lived access token travels in the response body now; the refresh token
// lives in an httpOnly cookie the browser stores and JS can't read.
type accessTokenBody struct {
	AccessToken string `json:"access_token"`
}

type registerRequest struct {
	Email       string           `json:"email"`
	Phone       string           `json:"phone"`
	CompanyName string           `json:"company_name"`
	LegalForm   models.LegalForm `json:"legal_form"`
	Password    string           `json:"password"`
	ToolIDs     []uuid.UUID      `json:"tool_ids"`
}

type registerResponse struct {
	User   *models.User    `json:"user"`
	Tokens accessTokenBody `json:"tokens"`
}

func (h *RegisterHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	user, sess, err := h.svc.Register(r.Context(), service.RegisterInput{
		Email:       req.Email,
		Phone:       req.Phone,
		CompanyName: req.CompanyName,
		LegalForm:   req.LegalForm,
		Password:    req.Password,
		ToolIDs:     req.ToolIDs,
	})
	if err != nil {
		writeServiceError(w, err)
		return
	}

	httpx.SetRefreshCookie(w, sess.RefreshToken, h.cookieSecure, sess.RefreshExpires)
	httpx.WriteJSON(w, http.StatusCreated, registerResponse{
		User:   user,
		Tokens: accessTokenBody{AccessToken: sess.AccessToken},
	})
}

type userLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *RegisterHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req userLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}

	user, sess, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	httpx.SetRefreshCookie(w, sess.RefreshToken, h.cookieSecure, sess.RefreshExpires)
	httpx.WriteJSON(w, http.StatusOK, registerResponse{
		User:   user,
		Tokens: accessTokenBody{AccessToken: sess.AccessToken},
	})
}

type refreshResponse struct {
	Tokens accessTokenBody `json:"tokens"`
}

// Refresh reads the refresh token from the httpOnly cookie (never the body),
// rotates it, and sets the new cookie. On failure the cookie is cleared so a
// dead session doesn't keep retrying.
func (h *RegisterHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken, ok := httpx.RefreshCookie(r)
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "missing refresh token")
		return
	}

	sess, err := h.svc.Refresh(r.Context(), refreshToken)
	if err != nil {
		if errors.Is(err, service.ErrRefreshAlreadyRotated) {
			httpx.WriteError(w, http.StatusConflict, "refresh_already_rotated", "refresh token was already rotated; retry")
			return
		}
		httpx.ClearRefreshCookie(w, h.cookieSecure)
		writeServiceError(w, err)
		return
	}

	httpx.SetRefreshCookie(w, sess.RefreshToken, h.cookieSecure, sess.RefreshExpires)
	httpx.WriteJSON(w, http.StatusOK, refreshResponse{Tokens: accessTokenBody{AccessToken: sess.AccessToken}})
}

// Logout revokes the current refresh token server-side and clears the cookie.
func (h *RegisterHandler) Logout(w http.ResponseWriter, r *http.Request) {
	if refreshToken, ok := httpx.RefreshCookie(r); ok {
		_ = h.svc.Logout(r.Context(), refreshToken)
	}
	httpx.ClearRefreshCookie(w, h.cookieSecure)
	w.WriteHeader(http.StatusNoContent)
}

type meResponse struct {
	User         *models.User                `json:"user"`
	Verification *models.VerificationRequest `json:"verification"`
}

type updateProfileRequest struct {
	Name      string           `json:"name"`
	LegalForm models.LegalForm `json:"legal_form"`
}

func (h *RegisterHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}
	var req updateProfileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_body", "malformed JSON body")
		return
	}
	user, verification, err := h.svc.UpdateProfile(r.Context(), userID, req.Name, req.LegalForm)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, meResponse{User: user, Verification: verification})
}

func (h *RegisterHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	user, verification, err := h.svc.GetMe(r.Context(), userID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, meResponse{User: user, Verification: verification})
}

func (h *RegisterHandler) UploadDocument(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "missing auth context")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_upload", "file too large or malformed multipart body")
		return
	}
	defer r.MultipartForm.RemoveAll()

	docType := models.DocumentType(r.FormValue("type"))

	file, header, err := r.FormFile("file")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid_upload", "file is required")
		return
	}
	file.Close()

	doc, err := h.svc.UploadDocument(r.Context(), userID, docType, header)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusCreated, doc)
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidCredentials):
		httpx.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "invalid email or password")
	case errors.Is(err, service.ErrEmailTaken):
		httpx.WriteError(w, http.StatusConflict, "email_taken", "email is already registered")
	case errors.Is(err, service.ErrInvalidInput):
		httpx.WriteError(w, http.StatusBadRequest, "invalid_input", err.Error())
	case errors.Is(err, service.ErrUserNotFound):
		httpx.WriteError(w, http.StatusUnauthorized, "unauthorized", "account not found")
	case errors.Is(err, service.ErrAccountNotEligible):
		httpx.WriteError(w, http.StatusForbidden, "account_not_eligible", "account status does not allow this action")
	case errors.Is(err, service.ErrUnsupportedFile):
		httpx.WriteError(w, http.StatusUnprocessableEntity, "unsupported_file", "unsupported file type")
	case errors.Is(err, service.ErrFileTooLarge):
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "file_too_large", "file exceeds maximum size")
	default:
		httpx.WriteError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}
