package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestFloorReputationTier(t *testing.T) {
	t.Parallel()
	tiers := []ReputationTier{
		{ID: "a", Name: "优质", Score: 100},
		{ID: "b", Name: "及格", Score: 60},
		{ID: "c", Name: "观察", Score: 50},
		{ID: "d", Name: "风险", Score: 40},
	}
	cases := []struct {
		score int
		want  string
	}{
		{100, "a"},
		{80, "b"},
		{60, "b"},
		{58, "c"},
		{41, "d"},
		{40, "d"},
		{0, "d"},
	}
	for _, tc := range cases {
		got := FloorReputationTier(tiers, tc.score)
		if got == nil || got.ID != tc.want {
			t.Fatalf("score %d: got %#v want %s", tc.score, got, tc.want)
		}
	}
}

func TestDefaultAccountReputationTierID(t *testing.T) {
	t.Parallel()
	tiers := []ReputationTier{
		{ID: "a", Name: "优质", Score: 100},
		{ID: "b", Name: "及格", Score: 60},
		{ID: "c", Name: "观察", Score: 50},
	}
	if got := DefaultAccountReputationTierID(tiers, 60); got != "b" {
		t.Fatalf("got %s", got)
	}
}

func TestReputationThresholdPoints(t *testing.T) {
	t.Parallel()
	if got := ReputationThresholdPoints(250, 100, 5); got != 10 {
		t.Fatalf("got %d", got)
	}
	if got := ReputationThresholdPoints(99, 100, 5); got != 0 {
		t.Fatalf("got %d", got)
	}
	if got := ReputationThresholdPoints(100, 0, 5); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestAccountReputationScoreEnabled(t *testing.T) {
	t.Parallel()
	if !AccountReputationScoreEnabled(AccountTypeOAuth, nil) {
		t.Fatal("oauth should deduct by default")
	}
	if AccountReputationScoreEnabled(AccountTypeAPIKey, nil) {
		t.Fatal("apikey should not deduct by default")
	}
	off := false
	if AccountReputationScoreEnabled(AccountTypeOAuth, &off) {
		t.Fatal("override off")
	}
}

func TestNormalizeReputationSettingsRejectsDuplicateScores(t *testing.T) {
	t.Parallel()
	in := DefaultReputationSettings()
	in.Enabled = true
	in.Tiers = []ReputationTier{
		{Name: "A", Score: 60},
		{Name: "B", Score: 60},
	}
	if _, err := NormalizeReputationSettings(in); err == nil {
		t.Fatal("expected duplicate score error")
	}
}

func TestFilterAccountsByReputationTier(t *testing.T) {
	t.Parallel()
	accounts := []Account{
		{ID: 1, ReputationTierID: "tier-pass"},
		{ID: 2, ReputationTierID: "tier-risk"},
		{ID: 3, ReputationTierID: "tier-pass"},
	}
	got := FilterAccountsByReputationTier(accounts, "tier-pass")
	if len(got) != 2 || got[0].ID != 1 || got[1].ID != 3 {
		t.Fatalf("got %#v", got)
	}
}

func TestApplySerializesConcurrentScoreChanges(t *testing.T) {
	t.Parallel()
	store := newMemReputationStore(map[int64]int{7: 60})
	svc := enabledReputationService(store)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.Apply(context.Background(), ReputationApplyInput{UserID: 7, Event: ReputationEventContentAudit})
		}()
	}
	wg.Wait()
	score, err := store.GetUserScore(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if score != 10 {
		t.Fatalf("score %d want 10", score)
	}
	if len(store.logs) != 10 {
		t.Fatalf("logs %d want 10", len(store.logs))
	}
	sum := 0
	for _, log := range store.logs {
		sum += log.Delta
		if log.ScoreAfter-log.ScoreBefore != log.Delta {
			t.Fatalf("log mismatch %#v", log)
		}
	}
	if sum != -50 {
		t.Fatalf("delta sum %d want -50", sum)
	}
}

func TestApplyClampedConcurrentDeductionsMatchLogs(t *testing.T) {
	t.Parallel()
	store := newMemReputationStore(map[int64]int{7: 8})
	svc := enabledReputationService(store)
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			svc.Apply(context.Background(), ReputationApplyInput{UserID: 7, Event: ReputationEventContentAudit})
		}()
	}
	wg.Wait()
	score, err := store.GetUserScore(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if score != 0 {
		t.Fatalf("score %d want 0", score)
	}
	sum := 0
	for _, log := range store.logs {
		sum += log.Delta
	}
	if sum != -8 {
		t.Fatalf("logged delta %d want -8", sum)
	}
}

func TestAdminSetScoreReturnsStoreError(t *testing.T) {
	t.Parallel()
	store := newMemReputationStore(nil)
	svc := enabledReputationService(store)
	err := svc.AdminSetScore(context.Background(), 7, 1, 40, "manual")
	if err == nil {
		t.Fatal("expected error when the user score cannot be written")
	}
	if len(store.logs) != 0 {
		t.Fatalf("logs %#v", store.logs)
	}
}

func TestAdminSetScoreWritesClampedScore(t *testing.T) {
	t.Parallel()
	store := newMemReputationStore(map[int64]int{7: 60})
	svc := enabledReputationService(store)
	svc.mu.Lock()
	svc.settings.Enabled = false
	svc.mu.Unlock()
	if err := svc.AdminSetScore(context.Background(), 7, 1, 150, "cap"); err != nil {
		t.Fatal(err)
	}
	score, err := store.GetUserScore(context.Background(), 7)
	if err != nil {
		t.Fatal(err)
	}
	if score != 100 {
		t.Fatalf("score %d want 100", score)
	}
	if len(store.logs) != 1 || store.logs[0].Event != ReputationEventAdminAdjust || store.logs[0].ScoreAfter != 100 {
		t.Fatalf("logs %#v", store.logs)
	}
}

func TestApplyAccountReputationUpdateOmitsUnspecifiedFields(t *testing.T) {
	t.Parallel()
	store := newMemReputationStore(nil)
	admin := &adminServiceImpl{reputationService: enabledReputationService(store)}
	on := true
	account := &Account{ID: 9, ReputationTierID: "tier-pass", ReputationScoreEnabled: &on}
	if err := admin.applyAccountReputation(context.Background(), account, nil, nil, false); err != nil {
		t.Fatal(err)
	}
	if len(store.patches) != 0 {
		t.Fatalf("unexpected patch %#v", store.patches)
	}
	if account.ReputationScoreEnabled == nil || !*account.ReputationScoreEnabled {
		t.Fatal("in-memory flag should stay set")
	}

	off := false
	if err := admin.applyAccountReputation(context.Background(), account, nil, &off, false); err != nil {
		t.Fatal(err)
	}
	if len(store.patches) != 1 {
		t.Fatalf("patches %d", len(store.patches))
	}
	patch := store.patches[0]
	if patch.TierID != nil || !patch.SetScoreEnabled || patch.ScoreEnabled == nil || *patch.ScoreEnabled {
		t.Fatalf("got %#v", patch)
	}
}

func TestApplyAccountReputationCreateWritesDefaultTier(t *testing.T) {
	t.Parallel()
	store := newMemReputationStore(nil)
	admin := &adminServiceImpl{reputationService: enabledReputationService(store)}
	account := &Account{ID: 3}
	empty := ""
	if err := admin.applyAccountReputation(context.Background(), account, &empty, nil, true); err != nil {
		t.Fatal(err)
	}
	if len(store.patches) != 1 {
		t.Fatalf("patches %d", len(store.patches))
	}
	patch := store.patches[0]
	if patch.TierID == nil || *patch.TierID != "tier-pass" || !patch.SetScoreEnabled || patch.ScoreEnabled != nil {
		t.Fatalf("got %#v", patch)
	}
}

func enabledReputationService(store ReputationStore) *ReputationService {
	svc := NewReputationService(nil, store)
	settings := DefaultReputationSettings()
	settings.Enabled = true
	svc.mu.Lock()
	svc.settings = settings
	svc.loadedAt = time.Now()
	svc.cacheTTL = time.Hour
	svc.mu.Unlock()
	return svc
}

type memReputationStore struct {
	mu      sync.Mutex
	scores  map[int64]int
	logs    []ReputationScoreLog
	patches []AccountReputationPatch
}

func newMemReputationStore(scores map[int64]int) *memReputationStore {
	if scores == nil {
		scores = map[int64]int{}
	}
	cloned := make(map[int64]int, len(scores))
	for id, score := range scores {
		cloned[id] = score
	}
	return &memReputationStore{scores: cloned}
}

func (m *memReputationStore) GetUserScore(_ context.Context, userID int64) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	score, ok := m.scores[userID]
	if !ok {
		return 0, errMemReputationMissing
	}
	return score, nil
}

func (m *memReputationStore) ApplyUserScoreChange(_ context.Context, userID int64, delta int, absolute *int) (int, int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	before, ok := m.scores[userID]
	if !ok {
		return 0, 0, errMemReputationMissing
	}
	after := ClampReputationScore(before + delta)
	if absolute != nil {
		after = ClampReputationScore(*absolute)
	}
	m.scores[userID] = after
	return before, after, nil
}

func (m *memReputationStore) InsertLog(_ context.Context, log ReputationScoreLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.logs = append(m.logs, log)
	return nil
}

func (m *memReputationStore) CountAccountsByTierID(context.Context, string) (int64, error) {
	return 0, nil
}

func (m *memReputationStore) SetAccountReputation(_ context.Context, _ int64, patch AccountReputationPatch) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.patches = append(m.patches, patch)
	return nil
}

var errMemReputationMissing = errors.New("reputation user missing")
