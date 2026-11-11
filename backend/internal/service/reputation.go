package service

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

const (
	SettingKeyReputationConfig = "reputation_config"

	ReputationMinScore = 0
	ReputationMaxScore = 100

	ReputationEventRegister           = "register"
	ReputationEventContentAudit       = "content_audit_block"
	ReputationEventOwnAccountSecurity = "own_account_security"
	ReputationEventUpstreamSecurity   = "upstream_security"
	ReputationEventRecharge           = "recharge"
	ReputationEventRedeemBalance      = "redeem_balance"
	ReputationEventAdminAdjust        = "admin_adjust"
)

var reputationBuiltinEvents = []string{
	ReputationEventRegister,
	ReputationEventContentAudit,
	ReputationEventOwnAccountSecurity,
	ReputationEventUpstreamSecurity,
	ReputationEventRecharge,
	ReputationEventRedeemBalance,
}

// ReputationTier is an admin-defined named score bucket. Accounts bind to ID.
type ReputationTier struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Score int    `json:"score"`
}

// ReputationEventRule is the per-event scoring rule stored in settings.
type ReputationEventRule struct {
	Enabled            bool    `json:"enabled"`
	Delta              int     `json:"delta"`
	ThresholdAmount    float64 `json:"threshold_amount,omitempty"`
	PointsPerThreshold int     `json:"points_per_threshold,omitempty"`
}

// ReputationSettings is the independent reputation-feature document.
type ReputationSettings struct {
	Enabled      bool                           `json:"enabled"`
	InitialScore int                            `json:"initial_score"`
	Tiers        []ReputationTier               `json:"tiers"`
	Events       map[string]ReputationEventRule `json:"events"`
}

// ReputationScoreLog is one ledger row.
type ReputationScoreLog struct {
	ID           int64
	UserID       int64
	Event        string
	Delta        int
	ScoreBefore  int
	ScoreAfter   int
	RequestID    string
	AccountID    *int64
	Amount       *float64
	ActorAdminID *int64
	Reason       string
}

// ReputationApplyInput is a scoring request.
type ReputationApplyInput struct {
	UserID       int64
	Event        string
	RequestID    string
	AccountID    *int64
	AccountType  string
	ScoreEnabled *bool
	Amount       float64
	ActorAdminID *int64
	Absolute     *int
	Reason       string
}

func defaultReputationConfigJSON() string {
	raw, err := MarshalReputationSettings(DefaultReputationSettings())
	if err != nil {
		return "{}"
	}
	return raw
}

func DefaultReputationSettings() ReputationSettings {
	return ReputationSettings{
		Enabled:      false,
		InitialScore: 60,
		Tiers: []ReputationTier{
			{ID: "tier-premium", Name: "优质", Score: 100},
			{ID: "tier-pass", Name: "及格", Score: 60},
			{ID: "tier-watch", Name: "观察", Score: 50},
			{ID: "tier-risk", Name: "风险", Score: 40},
		},
		Events: map[string]ReputationEventRule{
			ReputationEventRegister:           {Enabled: true, Delta: 60},
			ReputationEventContentAudit:       {Enabled: true, Delta: 5},
			ReputationEventOwnAccountSecurity: {Enabled: true, Delta: 20},
			ReputationEventUpstreamSecurity:   {Enabled: true, Delta: 5},
			ReputationEventRecharge:           {Enabled: true, ThresholdAmount: 100, PointsPerThreshold: 5},
			ReputationEventRedeemBalance:      {Enabled: true, ThresholdAmount: 100, PointsPerThreshold: 5},
		},
	}
}

func ClampReputationScore(score int) int {
	if score < ReputationMinScore {
		return ReputationMinScore
	}
	if score > ReputationMaxScore {
		return ReputationMaxScore
	}
	return score
}

func ReputationThresholdPoints(amount, threshold float64, pointsPer int) int {
	if threshold <= 0 || pointsPer <= 0 || amount <= 0 {
		return 0
	}
	return int(amount/threshold) * pointsPer
}

func OwnAccountReputationType(accountType string) bool {
	switch strings.TrimSpace(accountType) {
	case AccountTypeOAuth, AccountTypeSetupToken:
		return true
	default:
		return false
	}
}

func DefaultAccountReputationScoreEnabled(accountType string) bool {
	return OwnAccountReputationType(accountType)
}

func AccountReputationScoreEnabled(accountType string, override *bool) bool {
	if override != nil {
		return *override
	}
	return DefaultAccountReputationScoreEnabled(accountType)
}

func ResolveReputationSecurityEvent(accountType string) string {
	if OwnAccountReputationType(accountType) {
		return ReputationEventOwnAccountSecurity
	}
	return ReputationEventUpstreamSecurity
}

// FloorReputationTier returns the highest-score tier that is still <= userScore.
func FloorReputationTier(tiers []ReputationTier, userScore int) *ReputationTier {
	if len(tiers) == 0 {
		return nil
	}
	sorted := append([]ReputationTier(nil), tiers...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Score > sorted[j].Score })
	var matched *ReputationTier
	for i := range sorted {
		if sorted[i].Score <= userScore {
			matched = &sorted[i]
			break
		}
	}
	if matched != nil {
		copy := *matched
		return &copy
	}
	lowest := sorted[len(sorted)-1]
	for i := range sorted {
		if sorted[i].Score < lowest.Score {
			lowest = sorted[i]
		}
	}
	return &lowest
}

func DefaultAccountReputationTierID(tiers []ReputationTier, initialScore int) string {
	tier := FloorReputationTier(tiers, ClampReputationScore(initialScore))
	if tier == nil {
		return ""
	}
	return tier.ID
}

func ReputationTierByID(tiers []ReputationTier, id string) *ReputationTier {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	for i := range tiers {
		if tiers[i].ID == id {
			copy := tiers[i]
			return &copy
		}
	}
	return nil
}

func FilterAccountsByReputationTier(accounts []Account, tierID string) []Account {
	if strings.TrimSpace(tierID) == "" {
		return nil
	}
	out := make([]Account, 0, len(accounts))
	for _, account := range accounts {
		if strings.TrimSpace(account.ReputationTierID) == tierID {
			out = append(out, account)
		}
	}
	return out
}

func newReputationTierID() (string, error) {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "tier-" + hex.EncodeToString(buf), nil
}

func NormalizeReputationSettings(in ReputationSettings) (ReputationSettings, error) {
	out := DefaultReputationSettings()
	out.Enabled = in.Enabled
	out.InitialScore = ClampReputationScore(in.InitialScore)

	seenIDs := map[string]struct{}{}
	seenNames := map[string]struct{}{}
	seenScores := map[int]struct{}{}
	tiers := make([]ReputationTier, 0, len(in.Tiers))
	for _, tier := range in.Tiers {
		name := strings.TrimSpace(tier.Name)
		if name == "" {
			return ReputationSettings{}, infraerrors.BadRequest("INVALID_REPUTATION_TIER", "档位名称不能为空")
		}
		if tier.Score < ReputationMinScore || tier.Score > ReputationMaxScore {
			return ReputationSettings{}, infraerrors.BadRequest("INVALID_REPUTATION_TIER_SCORE", "档位分数必须在 0-100")
		}
		id := strings.TrimSpace(tier.ID)
		if id == "" {
			generated, err := newReputationTierID()
			if err != nil {
				return ReputationSettings{}, fmt.Errorf("generate reputation tier id: %w", err)
			}
			id = generated
		}
		if _, ok := seenIDs[id]; ok {
			return ReputationSettings{}, infraerrors.BadRequest("DUPLICATE_REPUTATION_TIER_ID", "档位 ID 不能重复")
		}
		nameKey := strings.ToLower(name)
		if _, ok := seenNames[nameKey]; ok {
			return ReputationSettings{}, infraerrors.BadRequest("DUPLICATE_REPUTATION_TIER_NAME", "档位名称不能重复")
		}
		if _, ok := seenScores[tier.Score]; ok {
			return ReputationSettings{}, infraerrors.BadRequest("DUPLICATE_REPUTATION_TIER_SCORE", "档位分数不能重复")
		}
		seenIDs[id] = struct{}{}
		seenNames[nameKey] = struct{}{}
		seenScores[tier.Score] = struct{}{}
		tiers = append(tiers, ReputationTier{ID: id, Name: name, Score: tier.Score})
	}
	if out.Enabled && len(tiers) == 0 {
		return ReputationSettings{}, infraerrors.BadRequest("REPUTATION_TIERS_REQUIRED", "开启信誉分调度时至少需要一个档位")
	}
	sort.SliceStable(tiers, func(i, j int) bool { return tiers[i].Score > tiers[j].Score })
	out.Tiers = tiers

	events := DefaultReputationSettings().Events
	for _, key := range reputationBuiltinEvents {
		rule := events[key]
		if incoming, ok := in.Events[key]; ok {
			rule.Enabled = incoming.Enabled
			switch key {
			case ReputationEventRegister:
				if incoming.Delta != 0 {
					rule.Delta = ClampReputationScore(incoming.Delta)
				} else {
					rule.Delta = out.InitialScore
				}
			case ReputationEventRecharge, ReputationEventRedeemBalance:
				rule.ThresholdAmount = incoming.ThresholdAmount
				rule.PointsPerThreshold = incoming.PointsPerThreshold
				if rule.Enabled {
					if rule.ThresholdAmount <= 0 {
						return ReputationSettings{}, infraerrors.BadRequest("INVALID_REPUTATION_THRESHOLD", "满足额度必须大于 0")
					}
					if rule.PointsPerThreshold < 0 {
						return ReputationSettings{}, infraerrors.BadRequest("INVALID_REPUTATION_POINTS", "加分不能为负数")
					}
				}
			default:
				if incoming.Delta < 0 {
					return ReputationSettings{}, infraerrors.BadRequest("INVALID_REPUTATION_DELTA", "扣分请填正整数")
				}
				rule.Delta = incoming.Delta
			}
		}
		if key == ReputationEventRegister {
			rule.Delta = out.InitialScore
		}
		events[key] = rule
	}
	out.Events = events
	return out, nil
}

func ParseReputationSettings(raw string) ReputationSettings {
	if strings.TrimSpace(raw) == "" {
		return DefaultReputationSettings()
	}
	var parsed ReputationSettings
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return DefaultReputationSettings()
	}
	normalized, err := NormalizeReputationSettings(parsed)
	if err != nil {
		return DefaultReputationSettings()
	}
	return normalized
}

func MarshalReputationSettings(settings ReputationSettings) (string, error) {
	normalized, err := NormalizeReputationSettings(settings)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func IsReputationSecurityEvent(event string) bool {
	return event == ReputationEventOwnAccountSecurity || event == ReputationEventUpstreamSecurity
}
