<script setup>
import {store} from '../store'
</script>

<template>
  <div class="toasts">
    <div
      v-for="t in store.toasts"
      :key="t.id"
      class="toast"
      :class="{gold: t.gold, out: t.out}"
    >{{ t.text }}</div>
  </div>
</template>
