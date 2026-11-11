package service

import (
	"context"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ctxkey"
	"github.com/gin-gonic/gin"
)

var reputationRuntime *ReputationService
var reputationAccountRepo AccountRepository

func SetReputationRuntime(svc *ReputationService, accounts AccountRepository) {
	reputationRuntime = svc
	reputationAccountRepo = accounts
}

func RecordReputationContentAudit(ctx context.Context, userID int64, requestID string) {
	if reputationRuntime == nil || userID <= 0 {
		return
	}
	reputationRuntime.ApplyContentAudit(ctx, userID, requestID)
}

func RecordReputationUpstreamSecurity(c *gin.Context) {
	if reputationRuntime == nil || c == nil || c.Request == nil {
		return
	}
	userID, _ := c.Request.Context().Value(ctxkey.UserID).(int64)
	if userID <= 0 {
		return
	}
	requestID, _ := c.Request.Context().Value(ctxkey.RequestID).(string)
	accountID, _ := c.Request.Context().Value(ctxkey.AccountID).(int64)
	if accountID <= 0 || reputationAccountRepo == nil {
		return
	}
	account, err := reputationAccountRepo.GetByID(c.Request.Context(), accountID)
	if err != nil || account == nil {
		return
	}
	reputationRuntime.ApplyUpstreamSecurity(c.Request.Context(), userID, strings.TrimSpace(requestID), account)
}
