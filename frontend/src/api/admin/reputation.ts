import { apiClient } from '../client'

export interface ReputationTier {
  id: string
  name: string
  score: number
}

export interface ReputationEventRule {
  enabled: boolean
  delta?: number
  threshold_amount?: number
  points_per_threshold?: number
}

export type ReputationEventKey =
  | 'register'
  | 'content_audit_block'
  | 'own_account_security'
  | 'upstream_security'
  | 'recharge'
  | 'redeem_balance'

export interface ReputationSettings {
  enabled: boolean
  initial_score: number
  tiers: ReputationTier[]
  events: Record<string, ReputationEventRule>
}

export const REPUTATION_EVENT_KEYS: ReputationEventKey[] = [
  'register',
  'content_audit_block',
  'own_account_security',
  'upstream_security',
  'recharge',
  'redeem_balance'
]

export const REPUTATION_THRESHOLD_EVENTS: ReputationEventKey[] = ['recharge', 'redeem_balance']

async function getSettings(): Promise<ReputationSettings> {
  const { data } = await apiClient.get<ReputationSettings>('/admin/reputation/settings')
  return data
}

async function updateSettings(payload: ReputationSettings): Promise<ReputationSettings> {
  const { data } = await apiClient.put<ReputationSettings>('/admin/reputation/settings', payload)
  return data
}

async function updateUserScore(
  userId: number,
  payload: { score: number; reason?: string }
): Promise<{ reputation_score: number; reputation_tier?: ReputationTier }> {
  const { data } = await apiClient.put<{ reputation_score: number; reputation_tier?: ReputationTier }>(
    `/admin/reputation/users/${userId}`,
    payload
  )
  return data
}

const reputationAPI = {
  getSettings,
  updateSettings,
  updateUserScore
}

export default reputationAPI
