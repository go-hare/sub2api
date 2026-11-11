<template>
  <AppLayout>
    <div class="space-y-6">
      <section class="card flex flex-wrap items-start justify-between gap-4 p-5">
        <div>
          <h1 class="text-xl font-semibold text-gray-900 dark:text-white">{{ t('admin.reputation.title') }}</h1>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">{{ t('admin.reputation.description') }}</p>
        </div>
        <button type="button" class="btn btn-primary" :disabled="saving || loading" @click="save">
          {{ saving ? t('common.submitting') : t('admin.reputation.save') }}
        </button>
      </section>

      <div class="space-y-6">
      <section class="card space-y-5 p-5">
        <label class="flex items-start gap-3">
          <input v-model="form.enabled" type="checkbox" class="mt-1 rounded border-gray-300" />
          <span>
            <span class="block font-medium text-gray-900 dark:text-white">{{ t('admin.reputation.enabled') }}</span>
            <span class="mt-1 block text-sm text-gray-500 dark:text-gray-400">{{ t('admin.reputation.enabledHint') }}</span>
          </span>
        </label>

        <div class="max-w-xs">
          <label class="input-label">{{ t('admin.reputation.initialScore') }}</label>
          <input v-model.number="form.initial_score" type="number" min="0" max="100" class="input" />
          <p class="input-hint">{{ t('admin.reputation.initialScoreHint') }}</p>
        </div>
      </section>

      <section class="card space-y-4 p-5">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.reputation.tiers.title') }}</h2>
            <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.reputation.tiers.description') }}</p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="addTier">{{ t('admin.reputation.tiers.add') }}</button>
        </div>

        <div v-if="form.tiers.length === 0" class="py-8 text-center text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.reputation.tiers.empty') }}
        </div>
        <div v-else class="space-y-3">
          <div
            v-for="(tier, index) in form.tiers"
            :key="tier.id || `new-${index}`"
            class="grid grid-cols-1 items-end gap-3 sm:grid-cols-[minmax(0,1fr)_8rem_auto]"
          >
            <label>
              <span class="input-label">{{ t('admin.reputation.tiers.name') }}</span>
              <input v-model="tier.name" type="text" class="input" />
            </label>
            <label>
              <span class="input-label">{{ t('admin.reputation.tiers.score') }}</span>
              <input v-model.number="tier.score" type="number" min="0" max="100" class="input" />
            </label>
            <button type="button" class="btn btn-secondary btn-sm" @click="removeTier(index)">
              {{ t('admin.reputation.tiers.remove') }}
            </button>
          </div>
        </div>
      </section>

      <section class="card space-y-4 p-5">
        <div>
          <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.reputation.events.title') }}</h2>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.reputation.events.description') }}</p>
        </div>
        <div class="space-y-4">
          <div
            v-for="key in eventKeys"
            :key="key"
            class="rounded-lg border border-gray-200 p-4 dark:border-dark-700"
          >
            <label class="flex items-start gap-3">
              <input v-model="eventRule(key).enabled" type="checkbox" class="mt-1 rounded border-gray-300" />
              <span>
                <span class="block font-medium text-gray-900 dark:text-white">{{ t(`admin.reputation.events.${key}`) }}</span>
                <span class="mt-1 block text-sm text-gray-500 dark:text-gray-400">{{ t(`admin.reputation.events.${key}Hint`) }}</span>
              </span>
            </label>
            <div v-if="key !== 'register'" class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
              <template v-if="isThresholdEvent(key)">
                <label>
                  <span class="input-label">{{ t('admin.reputation.events.thresholdAmount') }}</span>
                  <input v-model.number="eventRule(key).threshold_amount" type="number" min="0" step="0.01" class="input" />
                </label>
                <label>
                  <span class="input-label">{{ t('admin.reputation.events.pointsPerThreshold') }}</span>
                  <input v-model.number="eventRule(key).points_per_threshold" type="number" min="0" class="input" />
                </label>
              </template>
              <label v-else>
                <span class="input-label">{{ t('admin.reputation.events.delta') }}</span>
                <input v-model.number="eventRule(key).delta" type="number" min="0" class="input" />
              </label>
            </div>
          </div>
        </div>
      </section>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import AppLayout from '@/components/layout/AppLayout.vue'
import reputationAPI, {
  REPUTATION_EVENT_KEYS,
  REPUTATION_THRESHOLD_EVENTS,
  type ReputationEventKey,
  type ReputationEventRule,
  type ReputationSettings,
  type ReputationTier
} from '@/api/admin/reputation'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const saving = ref(false)
const eventKeys = REPUTATION_EVENT_KEYS

const form = reactive<ReputationSettings>({
  enabled: false,
  initial_score: 60,
  tiers: [],
  events: {}
})

function emptyRule(): ReputationEventRule {
  return { enabled: false, delta: 0, threshold_amount: 0, points_per_threshold: 0 }
}

function eventRule(key: ReputationEventKey): ReputationEventRule {
  if (!form.events[key]) {
    form.events[key] = emptyRule()
  }
  return form.events[key]
}

function isThresholdEvent(key: ReputationEventKey): boolean {
  return REPUTATION_THRESHOLD_EVENTS.includes(key)
}

function addTier() {
  form.tiers.push({ id: '', name: '', score: 0 })
}

function removeTier(index: number) {
  form.tiers.splice(index, 1)
}

function applySettings(settings: ReputationSettings) {
  form.enabled = settings.enabled
  form.initial_score = settings.initial_score
  form.tiers = (settings.tiers || []).map((tier: ReputationTier) => ({ ...tier }))
  form.events = {}
  for (const key of REPUTATION_EVENT_KEYS) {
    const incoming = settings.events?.[key] || emptyRule()
    form.events[key] = { ...emptyRule(), ...incoming }
  }
}

async function load() {
  loading.value = true
  try {
    applySettings(await reputationAPI.getSettings())
  } catch (error: unknown) {
    appStore.showError(reputationErrorMessage(error, t('admin.reputation.loadFailed')))
  } finally {
    loading.value = false
  }
}

function reputationErrorMessage(error: unknown, fallback: string): string {
  if (!error || typeof error !== 'object') {
    return fallback
  }
  const payload = error as {
    message?: string
    response?: { data?: { message?: string; detail?: string } }
  }
  return payload.message || payload.response?.data?.message || payload.response?.data?.detail || fallback
}

async function save() {
  saving.value = true
  try {
    applySettings(await reputationAPI.updateSettings({
      enabled: form.enabled,
      initial_score: Number(form.initial_score) || 0,
      tiers: form.tiers.map((tier) => ({
        id: tier.id,
        name: tier.name.trim(),
        score: Number(tier.score) || 0
      })),
      events: form.events
    }))
    appStore.showSuccess(t('admin.reputation.saved'))
  } catch (error: unknown) {
    appStore.showError(reputationErrorMessage(error, t('admin.reputation.saveFailed')))
  } finally {
    saving.value = false
  }
}

onMounted(() => {
  void load()
})
</script>
