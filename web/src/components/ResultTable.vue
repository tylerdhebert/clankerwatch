<script setup lang="ts">
import { BookOpen } from "lucide-vue-next";
import type { Annotation, ResultRow } from "../types";

const props = defineProps<{
  columns: string[];
  rows: ResultRow[];
  highlightedRows: Set<number>;
  notes: Annotation[];
}>();

const ranges = () =>
  props.notes
    .filter((item) => item.rowNumber && item.rowEnd && item.rowEnd !== item.rowNumber)
    .map((item) => ({
      start: item.rowNumber as number,
      end: item.rowEnd as number,
      note: item.note,
    }));

function annotationBracket(rowNumber: number) {
  const range = ranges().find((item) => rowNumber >= item.start && rowNumber <= item.end);
  if (!range) return "";
  if (rowNumber === range.start) return "bracket-start";
  if (rowNumber === range.end) return "bracket-end";
  return "bracket-mid";
}

function annotationBracketNote(rowNumber: number) {
  return ranges().find((item) => rowNumber === item.start)?.note ?? "";
}

function rowClasses(row: ResultRow) {
  const highlighted = row.highlight || props.highlightedRows.has(row.number);
  const previousHighlighted = props.highlightedRows.has(row.number - 1);
  const nextHighlighted = props.highlightedRows.has(row.number + 1);
  return {
    highlighted,
    "highlight-continues-from-prev": highlighted && previousHighlighted,
    "highlight-continues-to-next": highlighted && nextHighlighted,
  };
}
</script>

<template>
  <table>
    <thead>
      <tr>
        <th class="rownum">#</th>
        <th v-for="column in columns" :key="column">{{ column }}</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="row in rows" :key="row.number" :class="rowClasses(row)">
        <td class="rownum" :class="annotationBracket(row.number)">
          <span class="bracket-mark" aria-hidden="true"></span>
          <span v-if="annotationBracketNote(row.number)" class="bracket-note" tabindex="0">
            <BookOpen :size="12" />
            <span class="note-popper">{{ annotationBracketNote(row.number) }}</span>
          </span>
          <span>{{ row.number }}</span>
        </td>
        <td v-for="(cell, index) in row.cells" :key="index">{{ cell }}</td>
      </tr>
    </tbody>
  </table>
</template>
