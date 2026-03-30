<script setup lang="ts">
import { useRoute } from 'vue-router'
import AppLayout from '@/components/AppLayout.vue'
import { computed } from 'vue'

const route = useRoute()
const showLayout = computed(() => route.name !== 'Login')
</script>

<template>
  <AppLayout v-if="showLayout" />
  <router-view v-else />
</template>
