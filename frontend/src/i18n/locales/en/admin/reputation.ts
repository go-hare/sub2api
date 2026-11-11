export default {
  reputation: {
    title: 'Reputation Routing',
    description: 'Route requests to same-group accounts by user score. Independent from risk-control and content-audit settings.',
    nav: 'Reputation',
    save: 'Save settings',
    saved: 'Reputation settings saved',
    saveFailed: 'Failed to save reputation settings',
    loadFailed: 'Failed to load reputation settings',
    enabled: 'Enable reputation routing',
    enabledHint: 'When off, scheduling ignores tiers and related UI is hidden. Stored data is kept.',
    initialScore: 'Initial score on signup',
    initialScoreHint: 'Score written for new users. Range 0–100.',
    tiers: {
      title: 'Tiers',
      description: 'Accounts bind to a tier ID. Requests floor-map the user score and only use matching accounts.',
      add: 'Add tier',
      name: 'Name',
      score: 'Score floor',
      empty: 'No tiers yet',
      remove: 'Remove'
    },
    events: {
      title: 'Events',
      description: 'Only these events change the score. Admin edits are audited separately.',
      enabled: 'Enabled',
      delta: 'Deduct',
      thresholdAmount: 'Amount threshold',
      pointsPerThreshold: 'Points per threshold',
      register: 'Sign up',
      registerHint: 'Writes the initial score. It does not accumulate.',
      content_audit_block: 'Content audit block',
      content_audit_blockHint: 'Deduct when a request is blocked by content policy before upstream.',
      own_account_security: 'Own-account security event',
      own_account_securityHint: 'Deduct when an oauth / setup-token account hits a security restriction.',
      upstream_security: 'Upstream security event',
      upstream_securityHint: 'Deduct when an upstream account hits a security restriction.',
      recharge: 'Recharge',
      rechargeHint: 'Balance recharges add floor(amount / threshold) × points. Remainder is ignored.',
      redeem_balance: 'Redeem code (balance)',
      redeem_balanceHint: 'Balance-type codes only. Same threshold formula as recharge.'
    },
    account: {
      tier: 'Reputation tier',
      tierHint: 'This account only serves users whose floor-mapped tier matches.',
      scoreEnabled: 'Deduct on security events',
      scoreEnabledHint: 'When off, security events on this account do not change the user score. oauth / setup-token default on; upstream defaults off.'
    },
    users: {
      score: 'Reputation score',
      tier: 'Reputation tier',
      scoreHint: '0–100. The displayed tier is derived from the score.',
      invalidScore: 'Score must be an integer from 0 to 100',
      scoreUpdated: 'Reputation score updated'
    }
  }
}
