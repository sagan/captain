package admin

import (
	"errors"
	"net/http"

	"github.com/zeptop-dev/captain/internal/store"
)

func (h *handlers) changeUserID(w http.ResponseWriter, r *http.Request) {
	h.changeAccountID(w, r, false)
}
func (h *handlers) changeStaffID(w http.ResponseWriter, r *http.Request) {
	h.changeAccountID(w, r, true)
}
func (h *handlers) changeAccountID(w http.ResponseWriter, r *http.Request, staff bool) {
	actor := userFrom(r)
	if !actor.IsAdmin() {
		fail(w, http.StatusForbidden, "admin only")
		return
	}
	var in struct {
		ID int64 `json:"id"`
	}
	if !readJSON(w, r, &in) {
		return
	}
	oldID := idOf(r)
	if err := h.Store.ChangeAccountID(r.Context(), oldID, in.ID, staff); err != nil {
		switch {
		case errors.Is(err, store.ErrAccountID):
			fail(w, 400, err.Error())
		case errors.Is(err, store.ErrIDInUse):
			fail(w, 409, err.Error())
		case errors.Is(err, store.ErrNotFound):
			fail(w, 404, "account not found")
		default:
			serverErr(w, err)
		}
		return
	}
	if staff && actor.ID == oldID {
		actor.ID = in.ID
	}
	if !staff && oldID != in.ID {
		if h.State != nil {
			h.State.MoveUserID(oldID, in.ID)
		}
		if h.Dyn != nil {
			h.Dyn.MoveUserID(oldID, in.ID)
		}
	}
	ok(w, map[string]int64{"id": in.ID})
}
