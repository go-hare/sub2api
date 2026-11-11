import { computed, ref } from 'vue'
import reputationAPI, { type ReputationSettings, type ReputationTier } from '@/api/admin/reputation'

const settings = ref<ReputationSettings | null>(null)
const loaded = ref(false)
let inflight: Promise<ReputationSettings | null> | null = null

export function defaultReputationScoreEnabled(accountType?: string | null): boolean {
  return accountType === 'oauth' || accountType === 'setup-token'
}

export function defaultReputationTierID(settings: ReputationSettings | null | undefined): string {
  const tiers = settings?.tiers ?? []
  if (!tiers.length) {
    return ''
  }
  const score = Number.isFinite(settings?.initial_score) ? Number(settings?.initial_score) : 60
  const sorted = [...tiers].sort((a, b) => b.score - a.score)
  const matched = sorted.find((tier) => tier.score <= score)
  return (matched ?? sorted[sorted.length - 1]).id
}

export function useReputationSettings() {
  const enabled = computed(() => settings.value?.enabled === true)
  const tiers = computed<ReputationTier[]>(() => settings.value?.tiers ?? [])

  const load = async (force = false) => {
    if (loaded.value && !force) {
      return settings.value
    }
    if (inflight && !force) {
      return inflight
    }
    inflight = reputationAPI.getSettings()
      .then((value) => {
        settings.value = value
        loaded.value = true
        return value
      })
      .catch(() => {
        settings.value = null
        loaded.value = true
        return null
      })
      .finally(() => {
        inflight = null
      })
    return inflight
  }

  const tierOptions = computed(() =>
    tiers.value.map((tier) => ({
      value: tier.id,
      label: `${tier.name} (${tier.score})`
    }))
  )

  return {
    settings,
    enabled,
    tiers,
    tierOptions,
    defaultTierID: computed(() => defaultReputationTierID(settings.value)),
    load
  }
}
