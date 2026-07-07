<script setup>
import { onUnmounted } from 'vue'
import { useAppStore } from '@/stores/app'
import { useI18n } from 'vue-i18n'

const appStore = useAppStore()
const { t } = useI18n({ useScope: 'global' })
const working = ref(false)
const records = ref([])
const seenKeys = ref(new Set())
const host = ref('')
let abortController = markRaw(new AbortController())

const handlePingMessage = (e) => {
  const data = JSON.parse(e.data)
  const record = {
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

  const key = record.seq + '-' + record.host
  if (seenKeys.value.has(key)) return
  seenKeys.value.add(key)

  // Insert in ascending seq order so the table reads top-to-bottom
  // even when the backend emits replies out of order (go-ping's
  // sendPacket blocks on a 5s deadline, so several sends can burst
  // whose replies interleave on the way back). Within a single seq
  // the arrival order is preserved, so the ECMP/anycast replies
  // stay grouped.
  const list = records.value
  let insertAt = list.length
  for (let i = 0; i < list.length; i++) {
    if (list[i].seq > record.seq) {
      insertAt = i
      break
    }
  }
  list.splice(insertAt, 0, record)
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
  seenKeys.value = new Set()
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
        <tr v-for="record in records" :key="record.seq + '-' + record.from">
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
