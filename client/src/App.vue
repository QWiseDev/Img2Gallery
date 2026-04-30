<script setup>
import { computed } from 'vue'
import AdminView from './components/AdminView.vue'
import HomeView from './components/HomeView.vue'

const isAdminRoute = computed(() => window.location.pathname === '/admin')
</script>

<template>
  <AdminView v-if="isAdminRoute" />
  <HomeView v-else />
</template>
