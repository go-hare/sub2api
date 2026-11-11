package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type ReputationHandler struct {
	svc *service.ReputationService
}

func NewReputationHandler(svc *service.ReputationService) *ReputationHandler {
	return &ReputationHandler{svc: svc}
}

type updateReputationSettingsRequest struct {
	Enabled      bool                                   `json:"enabled"`
	InitialScore int                                    `json:"initial_score"`
	Tiers        []service.ReputationTier               `json:"tiers"`
	Events       map[string]service.ReputationEventRule `json:"events"`
}

type updateUserReputationRequest struct {
	Score  int    `json:"score"`
	Reason string `json:"reason"`
}

func (h *ReputationHandler) GetSettings(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.Success(c, service.DefaultReputationSettings())
		return
	}
	response.Success(c, h.svc.GetSettings(c.Request.Context()))
}

func (h *ReputationHandler) UpdateSettings(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.InternalError(c, "reputation service unavailable")
		return
	}
	var req updateReputationSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	updated, err := h.svc.UpdateSettings(c.Request.Context(), service.ReputationSettings{
		Enabled:      req.Enabled,
		InitialScore: req.InitialScore,
		Tiers:        req.Tiers,
		Events:       req.Events,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, updated)
}

func (h *ReputationHandler) UpdateUserScore(c *gin.Context) {
	if h == nil || h.svc == nil {
		response.InternalError(c, "reputation service unavailable")
		return
	}
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "Invalid user ID")
		return
	}
	var req updateUserReputationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.svc.AdminSetScore(c.Request.Context(), userID, getAdminIDFromContext(c), req.Score, strings.TrimSpace(req.Reason)); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	settings := h.svc.GetSettings(c.Request.Context())
	score := service.ClampReputationScore(req.Score)
	tier := service.FloorReputationTier(settings.Tiers, score)
	payload := gin.H{"reputation_score": score}
	if tier != nil {
		payload["reputation_tier"] = gin.H{"id": tier.ID, "name": tier.Name, "score": tier.Score}
	}
	response.Success(c, payload)
}
