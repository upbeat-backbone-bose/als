<script setup>
import { Copy as IconCopy } from '@vicons/carbon'

import { useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  value: String,
  text: Boolean,
  hideMessage: Boolean
})
const isClicked = ref(false)
const message = useMessage()
const { t } = useI18n({ useScope: 'global' })
const copy = async (value) => {
  try {
    await navigator.clipboard.writeText(value)
  } catch (error) {
    const textarea = document.createElement('textarea')
    document.body.appendChild(textarea)
    textarea.textContent = value
    textarea.select()
    document?.execCommand('copy')
    textarea.remove()
  }
  isClicked.value = true
  if (!props['hideMessage']) {
    message.info(t('copied_to_clipboard'))
  }
}

const handleUpdateShow = (show) => {
  if (!show) {
    setTimeout(() => (isClicked.value = false), 150)
  }
}
</script>

<template>
  <n-tooltip v-if="!props.text" placement="bottom" trigger="hover" @update:show="handleUpdateShow">
    <template #trigger>
      <div @click="copy(props.value)">
        <slot>
          <n-icon style="margin-left: 5px">
            <IconCopy />
          </n-icon>
        </slot>
      </div>
    </template>
    <span v-if="!isClicked">{{ t('copy_to_clipboard') }}</span>
    <span v-if="isClicked">{{ t('copied') }}</span>
  </n-tooltip>
  <n-button v-else text @click="copy(props.value)">
    <slot></slot>
  </n-button>
</template>
