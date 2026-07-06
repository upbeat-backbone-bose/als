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

const handlePingEnd = () => {
  // Backend signals natural completion of the ping run. Tear the
  // listener down and flip the button back to "Ping" without
  // touching abortController -- the fetch already returned 200 long
  // ago, so there's nothing to cancel.
  appStore.source.removeEventListener('Ping', handlePingMessage)
  appStore.source.removeEventListener('PingEnd', handlePingEnd)
  working.value = false
}

onUnmounted(() => {
  stopPing()
})

const stopPing = () => {
  appStore.source.removeEventListener('Ping', handlePingMessage)
  appStore.source.removeEventListener('PingEnd', handlePingEnd)
  // Only abort the in-flight request when the user actually asked
  // to stop (or we're unmounting). On natural completion handlePingEnd
  // runs first and we never get here; the request has long since
  // returned 200.
  abortController.abort('Unmounted')
  // Bug fix: previously stopPing left `working` at `true`, which
  // froze the button in the "Stop" state and made the next click
  // route back through stopPing instead of starting a new run.
  working.value = false
}

const ping = async () => {
  if (working.value) return false
  abortController = new AbortController()
  records.value = []
  working.value = true
  appStore.source.addEventListener('Ping', handlePingMessage)
  appStore.source.addEventListener('PingEnd', handlePingEnd)
  try {
    await appStore.requestMethod('ping', { ip: host.value }, abortController.signal)
  } catch {}
  // NOTE: do NOT call stopPing() here. /method/ping returns 200
  // immediately while the pinger runs in the background, so the
  // await resolves before any Ping frame is sent. Cleanup is driven
  // by the PingEnd SSE event (handlePingEnd) or by the user pressing
  // Stop (stopPing).
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
        <n-spin v-if="working" :size="16" class="ml-1"></n-spin>
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
          <td>
            <template v-if="record.latency !== '-'">{{ record.latency.toFixed(2) }} ms</template>
            <template v-else>- ms</template>
          </td>
        </tr>
      </tbody>
    </n-table>
  </n-space>
</template>
