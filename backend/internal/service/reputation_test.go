package service

import "testing"

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
