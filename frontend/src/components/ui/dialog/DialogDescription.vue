<script setup lang="ts">
import { type HTMLAttributes } from 'vue'
import { DialogDescription, type DialogDescriptionProps, useForwardProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<DialogDescriptionProps & { class?: HTMLAttributes['class'] }>()
const forwardedProps = useForwardProps(props)
</script>

<template>
  <DialogDescription
    v-bind="forwardedProps"
    :class="cn('text-sm text-neutral-400', props.class)"
  >
    <slot />
  </DialogDescription>
</template>
