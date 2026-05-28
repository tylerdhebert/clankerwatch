<script setup lang="ts">
import { ChevronFirst, ChevronLast, ChevronLeft, ChevronRight } from "lucide-vue-next";

const props = defineProps<{
  page: number;
  pageSize: number;
  total: number;
}>();

const emit = defineEmits<{
  change: [page: number];
}>();

function totalPages() {
  return Math.max(1, Math.ceil(props.total / props.pageSize));
}

function visibleStart() {
  return props.total === 0 ? 0 : (props.page - 1) * props.pageSize + 1;
}

function visibleEnd() {
  return Math.min(props.page * props.pageSize, props.total);
}

function go(page: number) {
  emit("change", Math.min(totalPages(), Math.max(1, page)));
}
</script>

<template>
  <nav class="pager" aria-label="pagination">
    <div class="pager-count">showing {{ visibleStart() }}-{{ visibleEnd() }} of {{ total }} rows</div>
    <div class="pager-controls">
      <button class="icon-button" :disabled="page <= 1" aria-label="first page" @click="go(1)">
        <ChevronFirst :size="16" />
      </button>
      <button class="icon-button" :disabled="page <= 1" aria-label="previous page" @click="go(page - 1)">
        <ChevronLeft :size="16" />
      </button>
      <span>page {{ page }} of {{ totalPages() }}</span>
      <button class="icon-button" :disabled="page >= totalPages()" aria-label="next page" @click="go(page + 1)">
        <ChevronRight :size="16" />
      </button>
      <button class="icon-button" :disabled="page >= totalPages()" aria-label="last page" @click="go(totalPages())">
        <ChevronLast :size="16" />
      </button>
    </div>
  </nav>
</template>
