package service

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type ReputationStore interface {
	GetUserScore(ctx context.Context, userID int64) (int, error)
	SetUserScore(ctx context.Context, userID int64, score int) error
	InsertLog(ctx context.Context, log ReputationScoreLog) error
	CountAccountsByTierID(ctx context.Context, tierID string) (int64, error)
	SetAccountReputation(ctx context.Context, accountID int64, tierID *string, scoreEnabled *bool) error
}

type ReputationService struct {
	mu          sync.RWMutex
	settings    ReputationSettings
	loadedAt    time.Time
	settingRepo SettingRepository
	store       ReputationStore
	cacheTTL    time.Duration
}

func NewReputationService(settingRepo SettingRepository, store ReputationStore) *ReputationService {
	return &ReputationService{
		settings:    DefaultReputationSettings(),
		settingRepo: settingRepo,
		store:       store,
		cacheTTL:    time.Second,
	}
}

func (s *ReputationService) GetSettings(ctx context.Context) ReputationSettings {
	if s == nil {
		return DefaultReputationSettings()
	}
	s.mu.RLock()
	if time.Since(s.loadedAt) < s.cacheTTL {
		out := s.settings
		s.mu.RUnlock()
		return out
	}
	s.mu.RUnlock()
	return s.refresh(ctx)
}

func (s *ReputationService) Enabled(ctx context.Context) bool {
	return s.GetSettings(ctx).Enabled
}

func (s *ReputationService) refresh(ctx context.Context) ReputationSettings {
	if s == nil || s.settingRepo == nil {
		return DefaultReputationSettings()
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyReputationConfig)
	if err != nil {
		s.mu.Lock()
		s.settings = DefaultReputationSettings()
		s.loadedAt = time.Now()
		out := s.settings
		s.mu.Unlock()
		return out
	}
	parsed := ParseReputationSettings(raw)
	s.mu.Lock()
	s.settings = parsed
	s.loadedAt = time.Now()
	s.mu.Unlock()
	return parsed
}

func (s *ReputationService) Invalidate() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.loadedAt = time.Time{}
	s.mu.Unlock()
}

func (s *ReputationService) UpdateSettings(ctx context.Context, incoming ReputationSettings) (ReputationSettings, error) {
	normalized, err := NormalizeReputationSettings(incoming)
	if err != nil {
		return ReputationSettings{}, err
	}
	if s.store != nil {
		current := s.GetSettings(ctx)
		currentIDs := map[string]struct{}{}
		for _, tier := range current.Tiers {
			currentIDs[tier.ID] = struct{}{}
		}
		nextIDs := map[string]struct{}{}
		for _, tier := range normalized.Tiers {
			nextIDs[tier.ID] = struct{}{}
		}
		for id := range currentIDs {
			if _, ok := nextIDs[id]; ok {
				continue
			}
			count, err := s.store.CountAccountsByTierID(ctx, id)
			if err != nil {
				return ReputationSettings{}, err
			}
			if count > 0 {
				return ReputationSettings{}, infraerrors.BadRequest("REPUTATION_TIER_IN_USE", "仍有账号占用该档位，无法删除")
			}
		}
	}
	raw, err := MarshalReputationSettings(normalized)
	if err != nil {
		return ReputationSettings{}, err
	}
	if err := s.settingRepo.Set(ctx, SettingKeyReputationConfig, raw); err != nil {
		return ReputationSettings{}, err
	}
	s.mu.Lock()
	s.settings = normalized
	s.loadedAt = time.Now()
	s.mu.Unlock()
	return normalized, nil
}

func (s *ReputationService) CurrentTier(ctx context.Context, score int) *ReputationTier {
	return FloorReputationTier(s.GetSettings(ctx).Tiers, ClampReputationScore(score))
}

func (s *ReputationService) FilterAccounts(ctx context.Context, userID int64, accounts []Account) ([]Account, error) {
	if s == nil || !s.Enabled(ctx) {
		return accounts, nil
	}
	if userID <= 0 {
		return accounts, nil
	}
	settings := s.GetSettings(ctx)
	if len(settings.Tiers) == 0 {
		return nil, ErrNoAvailableAccounts
	}
	score := settings.InitialScore
	if s.store != nil {
		stored, err := s.store.GetUserScore(ctx, userID)
		if err != nil {
			return nil, err
		}
		score = stored
	}
	tier := FloorReputationTier(settings.Tiers, score)
	if tier == nil {
		return nil, ErrNoAvailableAccounts
	}
	filtered := FilterAccountsByReputationTier(accounts, tier.ID)
	if len(filtered) == 0 {
		return nil, ErrNoAvailableAccounts
	}
	return filtered, nil
}

func (s *ReputationService) Apply(ctx context.Context, input ReputationApplyInput) {
	if s == nil || s.store == nil {
		return
	}
	settings := s.GetSettings(ctx)
	if !settings.Enabled && input.Event != ReputationEventAdminAdjust {
		return
	}
	if input.UserID <= 0 {
		return
	}
	if IsReputationSecurityEvent(input.Event) && !AccountReputationScoreEnabled(input.AccountType, input.ScoreEnabled) {
		return
	}
	delta, absolute, skip := reputationDelta(settings, input)
	if skip {
		return
	}
	before, err := s.store.GetUserScore(ctx, input.UserID)
	if err != nil {
		slog.Warn("reputation.get_score_failed", "user_id", input.UserID, "error", err)
		return
	}
	after := ClampReputationScore(before + delta)
	if absolute != nil {
		after = ClampReputationScore(*absolute)
		delta = after - before
	}
	if delta == 0 && input.Event != ReputationEventRegister && input.Event != ReputationEventAdminAdjust {
		return
	}
	if err := s.store.SetUserScore(ctx, input.UserID, after); err != nil {
		slog.Warn("reputation.set_score_failed", "user_id", input.UserID, "error", err)
		return
	}
	log := ReputationScoreLog{
		UserID:       input.UserID,
		Event:        input.Event,
		Delta:        delta,
		ScoreBefore:  before,
		ScoreAfter:   after,
		RequestID:    strings.TrimSpace(input.RequestID),
		AccountID:    input.AccountID,
		ActorAdminID: input.ActorAdminID,
		Reason:       strings.TrimSpace(input.Reason),
	}
	if input.Amount > 0 {
		amount := input.Amount
		log.Amount = &amount
	}
	if err := s.store.InsertLog(ctx, log); err != nil {
		slog.Warn("reputation.insert_log_failed", "user_id", input.UserID, "error", err)
	}
}

func reputationDelta(settings ReputationSettings, input ReputationApplyInput) (delta int, absolute *int, skip bool) {
	if input.Absolute != nil {
		value := ClampReputationScore(*input.Absolute)
		return 0, &value, false
	}
	rule, ok := settings.Events[input.Event]
	if !ok || !rule.Enabled {
		return 0, nil, true
	}
	switch input.Event {
	case ReputationEventRegister:
		value := ClampReputationScore(settings.InitialScore)
		return 0, &value, false
	case ReputationEventRecharge, ReputationEventRedeemBalance:
		return ReputationThresholdPoints(input.Amount, rule.ThresholdAmount, rule.PointsPerThreshold), nil, false
	default:
		if rule.Delta <= 0 {
			return 0, nil, true
		}
		return -rule.Delta, nil, false
	}
}

func (s *ReputationService) ApplyRegister(ctx context.Context, userID int64) {
	s.Apply(ctx, ReputationApplyInput{UserID: userID, Event: ReputationEventRegister})
}

func (s *ReputationService) HydrateUser(ctx context.Context, user *User) {
	if s == nil || user == nil || s.store == nil || user.ID <= 0 {
		return
	}
	stored, err := s.store.GetUserScore(ctx, user.ID)
	if err != nil {
		slog.Warn("reputation.hydrate_user_failed", "user_id", user.ID, "error", err)
		return
	}
	user.ReputationScore = stored
}

func (s *ReputationService) ApplyContentAudit(ctx context.Context, userID int64, requestID string) {
	s.Apply(ctx, ReputationApplyInput{UserID: userID, Event: ReputationEventContentAudit, RequestID: requestID})
}

func (s *ReputationService) ApplyUpstreamSecurity(ctx context.Context, userID int64, requestID string, account *Account) {
	if account == nil {
		return
	}
	accountID := account.ID
	s.Apply(ctx, ReputationApplyInput{
		UserID:       userID,
		Event:        ResolveReputationSecurityEvent(account.Type),
		RequestID:    requestID,
		AccountID:    &accountID,
		AccountType:  account.Type,
		ScoreEnabled: account.ReputationScoreEnabled,
	})
}

func (s *ReputationService) ApplyRecharge(ctx context.Context, userID int64, amount float64) {
	s.Apply(ctx, ReputationApplyInput{UserID: userID, Event: ReputationEventRecharge, Amount: amount})
}

func (s *ReputationService) ApplyRedeemBalance(ctx context.Context, userID int64, amount float64) {
	s.Apply(ctx, ReputationApplyInput{UserID: userID, Event: ReputationEventRedeemBalance, Amount: amount})
}

func (s *ReputationService) AdminSetScore(ctx context.Context, userID, actorAdminID int64, score int, reason string) error {
	if s == nil || s.store == nil {
		return infraerrors.ServiceUnavailable("REPUTATION_UNAVAILABLE", "reputation store unavailable")
	}
	value := ClampReputationScore(score)
	s.Apply(ctx, ReputationApplyInput{
		UserID:       userID,
		Event:        ReputationEventAdminAdjust,
		Absolute:     &value,
		ActorAdminID: &actorAdminID,
		Reason:       reason,
	})
	return nil
}

func (s *ReputationService) SetAccountFields(ctx context.Context, accountID int64, tierID *string, scoreEnabled *bool) error {
	if s == nil || s.store == nil {
		return nil
	}
	return s.store.SetAccountReputation(ctx, accountID, tierID, scoreEnabled)
}

func (s *ReputationService) ValidateAccountTier(ctx context.Context, tierID string) error {
	if strings.TrimSpace(tierID) == "" {
		return infraerrors.BadRequest("REPUTATION_TIER_REQUIRED", "请选择信誉档位")
	}
	if ReputationTierByID(s.GetSettings(ctx).Tiers, tierID) == nil {
		return infraerrors.BadRequest("INVALID_REPUTATION_TIER", "档位不存在")
	}
	return nil
}
