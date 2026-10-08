<script setup lang="ts">
import { type HTMLAttributes } from 'vue'
import { DialogTitle, type DialogTitleProps, useForwardProps } from 'reka-ui'
import { cn } from '@/lib/utils'

const props = defineProps<DialogTitleProps & { class?: HTMLAttributes['class'] }>()
const forwardedProps = useForwardProps(props)
</script>

<template>
  <DialogTitle
    v-bind="forwardedProps"
    :class="cn('text-lg font-semibold leading-none tracking-tight text-white', props.class)"
  >
    <slot />
  </DialogTitle>
</template>
