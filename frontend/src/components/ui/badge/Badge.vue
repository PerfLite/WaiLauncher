<script setup lang="ts">
import type { HTMLAttributes } from 'vue'
import { cn } from '@/lib/utils'
import { type BadgeVariants, badgeVariants } from './badge'

interface Props {
  variant?: BadgeVariants['variant']
  class?: HTMLAttributes['class']
}

const props = defineProps<Props>()
</script>

<template>
  <div :class="cn(badgeVariants({ variant: props.variant }), props.class)">
    <slot />
  </div>
</template>
