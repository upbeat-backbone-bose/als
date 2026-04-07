<script setup>
import { onUnmounted } from 'vue'
import { useAppStore } from '@/stores/app'
import { useI18n } from 'vue-i18n'

const appStore = useAppStore()
const { t } = useI18n({ useScope: 'global' })
const working = ref(false)
const records = ref([])
const host = ref('')
let abortController = markRaw(new AbortController())

const handlePingMessage = (e) => {
  const data = JSON.parse(e.data)
  let record = {
    host: '-',
    seq: data.seq,
    ttl: '-',
    latency: '-'
  }

  if (!data.is_timeout) {
    record.host = data.from
    record.ttl = data.ttl
    record.latency = data.latency / 1000000
  }

  records.value.push(record)
  return
}

onUnmounted(() => {
  stopPing()
})

const stopPing = () => {
  appStore.source.removeEventListener('Ping', handlePingMessage)
  abortController.abort('Unmounted')
}

const ping = async () => {
  if (working.value) return false
  abortController = new AbortController()
  records.value = []
  working.value = true
  appStore.source.addEventListener('Ping', handlePingMessage)
  try {
    await appStore.requestMethod('ping', { ip: host.value }, abortController.signal)
  } catch (e) {}
  stopPing()
  working.value = false
}
</script>

<template>
  <n-space vertical>
    <n-input-group>
      <n-input
        :disabled="working"
        v-model:value="host"
        :placeholder="t('ping_placeholder')"
        @keyup.enter="ping"
      />
      <n-button :type="working ? 'error' : 'primary'" ghost @click="working ? stopPing() : ping()">
        <template v-if="working"> {{ t('stop') }} </template>
        <template v-else> {{ t('tool_ping') }} </template>
        <n-spin v-if="working" :size="16" style="margin-left: 5px"></n-spin>
      </n-button>
    </n-input-group>
    <n-table v-show="records.length > 0" :bordered="false" :single-line="false">
      <thead>
        <tr>
          <th>#</th>
          <th>{{ t('host') }}</th>
          <th>{{ t('ttl') }}</th>
          <th>{{ t('latency') }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="record in records">
          <td>{{ record.seq }}</td>
          <td>{{ record.host }}</td>
          <td>{{ record.ttl }}</td>
          <td>{{ record.latency.toFixed(2) }} ms</td>
        </tr>
      </tbody>
    </n-table>
  </n-space>
</template>
