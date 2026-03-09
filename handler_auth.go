package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
)

// handleRegister handles POST /auth/register
func (h *handler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !h.registrationEnabled {
		respondError(w, http.StatusForbidden, "registration is currently disabled")
		return
	}

	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, token, err := h.auth.Register(req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, ErrEmailRequired),
			errors.Is(err, ErrPasswordTooShort),
			errors.Is(err, ErrPasswordTooLong),
			errors.Is(err, ErrEmailTaken):
			respondError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			log.Printf("register error: %v", err)
			respondError(w, http.StatusInternalServerError, "registration failed")
		}
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// handleLogin handles POST /auth/login
func (h *handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	user, token, err := h.auth.Login(req.Email, req.Password)
	if err != nil {
		if errors.Is(err, ErrInvalidCreds) {
			respondError(w, http.StatusUnauthorized, err.Error())
		} else {
			log.Printf("login error: %v", err)
			respondError(w, http.StatusInternalServerError, "login failed")
		}
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// handleConfig returns public configuration (e.g. whether registration is enabled).
// GET /api/config — unauthenticated.
func (h *handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	respondJSON(w, http.StatusOK, map[string]interface{}{
		"registration_enabled": h.registrationEnabled,
	})
}
