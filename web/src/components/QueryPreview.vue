<script setup lang="ts">
import { computed } from "vue";
import { highlightSQL } from "../sql";

const props = defineProps<{
  query: string;
}>();

const highlighted = computed(() => highlightSQL(props.query));
</script>

<template>
  <pre class="sql-preview" v-html="highlighted"></pre>
</template>
