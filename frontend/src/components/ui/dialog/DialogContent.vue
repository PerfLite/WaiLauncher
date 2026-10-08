<script setup lang="ts">
import { type HTMLAttributes } from 'vue'
import {
  DialogClose,
  DialogContent,
  type DialogContentEmits,
  type DialogContentProps,
  DialogPortal,
  useForwardPropsEmits,
} from 'reka-ui'
import DialogOverlay from './DialogOverlay.vue'
import { cn } from '@/lib/utils'

interface Props extends DialogContentProps {
  class?: HTMLAttributes['class']
  showClose?: boolean
}

const props = withDefaults(defineProps<Props>(), {
  showClose: true,
})
const emits = defineEmits<DialogContentEmits>()

const forwarded = useForwardPropsEmits(props, emits)
</script>

<template>
  <DialogPortal>
    <DialogOverlay />
    <DialogContent
      v-bind="forwarded"
      :class="
        cn(
          'fixed left-[50%] top-[50%] z-50 grid w-full max-w-lg translate-x-[-50%] translate-y-[-50%] gap-4 border border-white/10 bg-[#161922] p-6 shadow-2xl duration-200 data-[state=open]:animate-in data-[state=closed]:animate-out data-[state=closed]:fade-out-0 data-[state=open]:fade-in-0 data-[state=closed]:zoom-out-95 data-[state=open]:zoom-in-95 data-[state=closed]:slide-out-to-left-1/2 data-[state=closed]:slide-out-to-top-[48%] data-[state=open]:slide-in-from-left-1/2 data-[state=open]:slide-in-from-top-[48%] rounded-2xl',
          props.class
        )
      "
    >
      <slot />

      <DialogClose
        v-if="props.showClose"
        class="absolute right-4 top-4 rounded-lg p-1.5 text-neutral-400 transition-colors hover:bg-white/10 hover:text-white focus:outline-none"
      >
        <svg class="h-4 w-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"><path d="M18 6 6 18M6 6l12 12"/></svg>
        <span class="sr-only">Close</span>
      </DialogClose>
    </DialogContent>
  </DialogPortal>
</template>
