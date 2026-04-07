<script setup>
import { useAppStore } from '@/stores/app'
import { useI18n } from 'vue-i18n'
const appStore = useAppStore()
import { shallowRef, computed, defineAsyncComponent, onMounted } from 'vue'
import { storeToRefs } from 'pinia'
const { config } = storeToRefs(appStore)
const { t } = useI18n({ useScope: 'global' })
const _v = (loader) => {
  return defineAsyncComponent({
    loader: loader,
    delay: 200
  })
}

const tools = ref([
  {
    labelKey: 'tool_ping',
    show: false,
    enable: false,
    configKey: 'feature_ping',
    componentNode: _v(() => import('./Utilities/Ping.vue'))
  },
  {
    labelKey: 'tool_iperf3',
    show: false,
    enable: false,
    configKey: 'feature_iperf3',
    componentNode: _v(() => import('./Utilities/IPerf3.vue'))
  },
  {
    labelKey: 'tool_speedtest_net',
    show: false,
    enable: false,
    configKey: 'feature_speedtest_dot_net',
    componentNode: _v(() => import('./Utilities/SpeedtestNet.vue'))
  },
  {
    labelKey: 'tool_shell',
    show: false,
    enable: false,
    configKey: 'feature_shell',
    componentNode: _v(() => import('./Utilities/Shell.vue'))
  }
])

onMounted(() => {
  for (var tool of tools.value) {
    tool.enable = config.value[tool.configKey] ?? false
  }
})

const toolComponent = shallowRef(null)

const toolComponentShow = computed({
  get() {
    for (const tool of tools.value) {
      if (tool.show) {
        toolComponent.value = toRaw(tool.componentNode)
        return true
      }
    }
    return false
  },
  set(newValue) {
    if (newValue) return
    for (const tool of tools.value) {
      if (tool.show) {
        tool.show = false
        return
      }
    }
  }
})

const hasToolEnable = computed(() => {
  for (const tool of tools.value) {
    if (tool.enable) {
      return true
    }
  }
  return false
})

const toolComponentLabel = computed(() => {
  for (const tool of tools.value) {
    if (tool.show) {
      return t(tool.labelKey)
    }
  }
  return ''
})

const handleDrawClosed = () => {
  toolComponent.value = null
}
</script>

<template>
  <n-card v-if="hasToolEnable">
    <template #header> {{ $t('network_tools') }} </template>
    <n-space>
      <template v-for="tool in tools">
        <n-button v-if="tool.enable" @click="tool.show = true">{{ t(tool.labelKey) }}</n-button>
      </template>
    </n-space>
  </n-card>
  <n-drawer
    v-model:show="toolComponentShow"
    :width="appStore.drawerWidth"
    placement="right"
    @after-leave="handleDrawClosed"
  >
    <n-drawer-content :title="toolComponentLabel" closable>
      <component :is="toolComponent" @closed="toolComponentShow = false" />
    </n-drawer-content>
  </n-drawer>
</template>
