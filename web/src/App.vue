<script setup lang="ts">
import { computed, onMounted, reactive, ref } from "vue";
import {
  BookOpen,
  CircleAlert,
  FileText,
  Highlighter,
  ListChecks,
  Moon,
  RefreshCw,
  Save,
  Search,
  Settings,
  ShieldCheck,
  Sun,
  Terminal,
  TowerControl,
  X,
} from "lucide-vue-next";
import Pager from "./components/Pager.vue";
import QueryPreview from "./components/QueryPreview.vue";
import ResultTable from "./components/ResultTable.vue";
import type { AgentSession, Annotation, Page, Profile, ResultRow, Run, RunSummary } from "./types";

const apiBase = window.location.hostname === "cwatch.localhost" ? `${window.location.protocol}//cwatchapi.localhost` : "http://127.0.0.1:48731";
const profiles = ref<Profile[]>([]);
const sessions = ref<AgentSession[]>([]);
const selectedSessionId = ref("all");
const runs = ref<RunSummary[]>([]);
const selectedRun = ref<Run | null>(null);
const rows = ref<Page<ResultRow>>({ items: [], page: 1, pageSize: 50, total: 0 });
const activeTab = ref<"table" | "stdout" | "stderr">("table");
const statusText = ref("connecting");
const errorText = ref("");
const profileModalOpen = ref(false);
const darkMode = ref(loadTheme());

const profileForm = reactive({
  name: "local-sqlite",
  adapter: "sqlite",
  command: "",
  argsText: "",
  envText: "",
  secretEnvText: "",
  timeoutMs: 30000,
  maxRows: 1000,
});

const selectedSummary = computed(() => runs.value.find((run) => run.id === selectedRun.value?.id));
const highlightedRows = computed(() => {
  const rows = new Set<number>();
  for (const item of selectedRun.value?.highlights ?? []) {
    if (!item.rowNumber) continue;
    const end = item.rowEnd ?? item.rowNumber;
    for (let row = item.rowNumber; row <= end; row++) rows.add(row);
  }
  return rows;
});

async function apiJSON<T>(path: string, options: RequestInit = {}): Promise<T> {
  errorText.value = "";
  const response = await fetch(`${apiBase}${path}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...(options.headers || {}),
    },
  });
  const text = await response.text();
  const payload = text ? JSON.parse(text) : null;
  if (!response.ok) {
    throw new Error(payload?.error || response.statusText);
  }
  return payload as T;
}

async function refreshAll() {
  try {
    await Promise.all([loadProfiles(), loadSessions(), loadRuns()]);
    statusText.value = "connected";
  } catch (error) {
    statusText.value = "offline";
    errorText.value = error instanceof Error ? error.message : String(error);
  }
}

async function loadProfiles() {
  profiles.value = await apiJSON<Profile[]>("/api/profiles");
}

async function loadSessions() {
  sessions.value = await apiJSON<AgentSession[]>("/api/sessions");
}

async function loadRuns() {
  const query = selectedSessionId.value === "all" ? "" : `?sessionId=${encodeURIComponent(selectedSessionId.value)}`;
  runs.value = await apiJSON<RunSummary[]>(`/api/runs${query}`);
  if (selectedRun.value && !runs.value.some((run) => run.id === selectedRun.value?.id)) {
    selectedRun.value = null;
    rows.value = { items: [], page: 1, pageSize: rows.value.pageSize, total: 0 };
  }
  if (!selectedRun.value && runs.value.length) {
    await selectRun(runs.value[0].id);
  } else if (selectedRun.value) {
    await selectRun(selectedRun.value.id, false);
  }
}

async function selectSession(id: string) {
  selectedSessionId.value = id;
  await loadRuns();
}

async function selectRun(id: number, setTab = true) {
  selectedRun.value = await apiJSON<Run>(`/api/runs/${id}`);
  if (setTab) activeTab.value = selectedRun.value.columns.length ? "table" : "stdout";
  await loadRows(1);
}

async function loadRows(page: number) {
  if (!selectedRun.value) return;
  rows.value = await apiJSON<Page<ResultRow>>(`/api/runs/${selectedRun.value.id}/rows?page=${page}&pageSize=${rows.value.pageSize}`);
}

async function saveProfile() {
  const profile = await apiJSON<Profile>("/api/profiles", {
    method: "POST",
    body: JSON.stringify({
      name: profileForm.name.trim(),
      adapter: profileForm.adapter,
      command: profileForm.command.trim(),
      args: lines(profileForm.argsText),
      env: keyValues(profileForm.envText),
      timeoutMs: profileForm.timeoutMs,
      maxRows: profileForm.maxRows,
    }),
  });
  pickProfile(profile);
  await loadProfiles();
}

async function unlockProfile() {
  if (!profileForm.name.trim()) return;
  await apiJSON(`/api/profiles/${encodeURIComponent(profileForm.name.trim())}/unlock`, {
    method: "POST",
    body: JSON.stringify({ secretEnv: keyValues(profileForm.secretEnvText) }),
  });
  profileForm.secretEnvText = "";
  await loadProfiles();
}

async function goToHighlight(rowNumber?: number) {
  if (!rowNumber) return;
  activeTab.value = "table";
  await loadRows(Math.ceil(rowNumber / rows.value.pageSize));
}

function rowLabel(annotation: Annotation) {
  if (!annotation.rowNumber) return "run";
  if (annotation.rowEnd && annotation.rowEnd !== annotation.rowNumber) {
    return `rows ${annotation.rowNumber}-${annotation.rowEnd}`;
  }
  return `row ${annotation.rowNumber}`;
}

function lines(value: string) {
  return value.split(/\r?\n/).map((line) => line.trim()).filter(Boolean);
}

function keyValues(value: string) {
  const result: Record<string, string> = {};
  for (const line of lines(value)) {
    const splitAt = line.indexOf("=");
    if (splitAt > 0) {
      result[line.slice(0, splitAt).trim()] = line.slice(splitAt + 1);
    }
  }
  return result;
}

function formatTime(value: string) {
  return new Intl.DateTimeFormat(undefined, {
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
  }).format(new Date(value));
}

function pickProfile(profile: Profile) {
  profileForm.name = profile.name;
  profileForm.adapter = profile.adapter;
  profileForm.command = profile.command;
  profileForm.argsText = profile.args.join("\n");
  profileForm.envText = Object.entries(profile.env).map(([key, value]) => `${key}=${value}`).join("\n");
  profileForm.timeoutMs = profile.timeoutMs;
  profileForm.maxRows = profile.maxRows;
}

function openProfiles() {
  profileModalOpen.value = true;
}

function loadTheme() {
  const saved = window.localStorage.getItem("cwatch-theme");
  if (saved === "dark") return true;
  if (saved === "light") return false;
  return window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? false;
}

function toggleTheme() {
  darkMode.value = !darkMode.value;
  window.localStorage.setItem("cwatch-theme", darkMode.value ? "dark" : "light");
}

function connectEvents() {
  const events = new EventSource(`${apiBase}/api/events`);
  events.onopen = () => {
    statusText.value = "live";
  };
  events.onerror = () => {
    statusText.value = "reconnecting";
  };
  events.addEventListener("update", async () => {
    await loadRuns();
    await loadProfiles();
    await loadSessions();
  });
}

onMounted(async () => {
  await refreshAll();
  connectEvents();
});
</script>

<template>
  <div class="shell" :class="{ dark: darkMode }">
    <header class="topbar">
      <div class="brand">
        <TowerControl :size="22" />
        <div>
          <strong>clankerwatch</strong>
        </div>
      </div>
      <div class="topbar-actions">
        <div class="status-pill" :class="statusText">
          <ShieldCheck :size="16" />
          <span>{{ statusText }}</span>
        </div>
        <button class="topbar-button" @click="openProfiles">
          <Settings :size="16" />
          <span>profiles</span>
        </button>
        <button class="icon-button" :aria-label="darkMode ? 'use light mode' : 'use dark mode'" @click="toggleTheme">
          <Sun v-if="darkMode" :size="16" />
          <Moon v-else :size="16" />
        </button>
      </div>
    </header>

    <main class="workspace">
      <aside class="sidebar">
        <section class="panel sessions">
          <div class="panel-title">
            <Terminal :size="16" />
            <span>sessions</span>
          </div>
          <button class="session-item" :class="{ selected: selectedSessionId === 'all' }" @click="selectSession('all')">
            <strong>all sessions</strong>
            <small>{{ runs.length }} visible runs</small>
          </button>
          <button
            v-for="session in sessions"
            :key="session.id"
            class="session-item"
            :class="{ selected: selectedSessionId === session.id }"
            @click="selectSession(session.id)"
          >
            <strong>{{ session.name }}</strong>
            <small>{{ session.id }}</small>
          </button>
        </section>

        <section class="panel runs">
          <div class="panel-title">
            <Search :size="16" />
            <span>runs</span>
          </div>
          <button
            v-for="run in runs"
            :key="run.id"
            class="run-item"
            :class="{ selected: selectedSummary?.id === run.id }"
            @click="selectRun(run.id)"
          >
            <span class="run-id">#{{ run.id }}</span>
            <span class="run-text">
              <strong>{{ run.reason }}</strong>
              <small>{{ run.profile }} / {{ formatTime(run.startedAt) }}</small>
            </span>
          </button>
        </section>
      </aside>

      <section class="center">
        <div v-if="errorText" class="error-strip">
          <CircleAlert :size="16" />
          <span>{{ errorText }}</span>
        </div>

        <section class="run-overview">
          <div v-if="selectedRun" class="query-head">
            <div>
              <span class="eyebrow">selected run</span>
              <h1>{{ selectedRun.profile }}</h1>
              <p>{{ selectedRun.reason }}</p>
            </div>
          </div>
          <QueryPreview v-if="selectedRun" :query="selectedRun.query" />
          <div v-else class="overview-empty">waiting for an agent run</div>
        </section>

        <section class="run-surface">
          <div class="run-tabs">
            <button :class="{ active: activeTab === 'table' }" @click="activeTab = 'table'">
              <ListChecks :size="16" />
              <span>table</span>
            </button>
            <button :class="{ active: activeTab === 'stdout' }" @click="activeTab = 'stdout'">
              <Terminal :size="16" />
              <span>stdout</span>
            </button>
            <button :class="{ active: activeTab === 'stderr' }" @click="activeTab = 'stderr'">
              <FileText :size="16" />
              <span>stderr</span>
            </button>
          </div>

          <div v-if="selectedRun" class="run-context">
            <div>
              <span class="eyebrow">run {{ selectedRun.id }}</span>
              <h2>{{ selectedRun.reason }}</h2>
            </div>
          </div>

          <div v-if="activeTab === 'table'" class="table-wrap">
            <div v-if="selectedRun?.parseError" class="empty-state">{{ selectedRun.parseError }}</div>
            <ResultTable v-else-if="selectedRun && rows.items.length" :columns="selectedRun.columns" :rows="rows.items" :highlighted-rows="highlightedRows" :notes="selectedRun.notes" />
            <div v-else class="empty-state">no parsed rows</div>
          </div>

          <pre v-if="activeTab === 'stdout'" class="raw-output">{{ selectedRun?.stdout || "" }}</pre>
          <pre v-if="activeTab === 'stderr'" class="raw-output">{{ selectedRun?.stderr || "" }}</pre>

          <Pager v-if="activeTab === 'table' && selectedRun" :page="rows.page" :page-size="rows.pageSize" :total="rows.total" @change="loadRows" />
        </section>
      </section>

      <aside class="rightbar">
        <section class="panel notes" v-if="selectedRun">
          <div class="panel-title">
            <BookOpen :size="16" />
            <span>notes</span>
          </div>
          <div v-if="selectedRun.notes.length" class="annotation-list">
            <p v-for="note in selectedRun.notes" :key="note.id">
              <strong v-if="note.rowNumber">{{ rowLabel(note) }}</strong>
              <span>{{ note.note }}</span>
            </p>
          </div>
          <div v-else class="quiet-empty">no agent notes yet</div>
        </section>

        <section class="panel notes" v-if="selectedRun">
          <div class="panel-title">
            <Highlighter :size="16" />
            <span>highlights</span>
          </div>
          <div v-if="selectedRun.highlights.length" class="annotation-list">
            <button v-for="item in selectedRun.highlights" :key="item.id" class="highlight-link" @click="goToHighlight(item.rowNumber)">
              <strong>{{ rowLabel(item) }}</strong>
              <span>{{ item.note }}</span>
            </button>
          </div>
          <div v-else class="quiet-empty">no highlighted rows yet</div>
        </section>
      </aside>
    </main>

    <div v-if="profileModalOpen" class="modal-backdrop" @click.self="profileModalOpen = false">
      <section class="profile-modal" role="dialog" aria-modal="true" aria-label="profiles">
        <header class="modal-head">
          <div class="panel-title">
            <TowerControl :size="16" />
            <span>profiles</span>
          </div>
          <button class="icon-button" aria-label="close profiles" @click="profileModalOpen = false">
            <X :size="16" />
          </button>
        </header>

        <div class="profile-modal-body">
          <section class="profile-list">
            <button
              v-for="profile in profiles"
              :key="profile.name"
              class="profile-item"
              :class="{ selected: profile.name === profileForm.name }"
              @click="pickProfile(profile)"
            >
              <span>
                <strong>{{ profile.name }}</strong>
                <small>{{ profile.adapter }}</small>
              </span>
              <span class="unlock-dot" :class="{ on: profile.unlocked }"></span>
            </button>
          </section>

          <section class="profile-editor">
            <label>
              <span>name</span>
              <input v-model="profileForm.name" placeholder="prod-readonly" />
            </label>

            <label>
              <span>adapter</span>
              <select v-model="profileForm.adapter">
                <option value="sqlite">sqlite</option>
                <option value="postgres">postgres</option>
                <option value="sqlserver">sql server</option>
                <option value="generic">generic</option>
              </select>
            </label>

            <label>
              <span>command</span>
              <input v-model="profileForm.command" placeholder="sqlite3, psql, sqlcmd" />
            </label>

            <label>
              <span>args</span>
              <textarea v-model="profileForm.argsText" rows="4" placeholder="one arg per line&#10;use {query_file} if needed"></textarea>
            </label>

            <div class="inline-fields">
              <label>
                <span>timeout ms</span>
                <input v-model.number="profileForm.timeoutMs" type="number" min="1000" />
              </label>
              <label>
                <span>max rows</span>
                <input v-model.number="profileForm.maxRows" type="number" min="1" />
              </label>
            </div>

            <label>
              <span>env</span>
              <textarea v-model="profileForm.envText" rows="3" placeholder="KEY=value"></textarea>
            </label>

            <div class="button-row">
              <button class="primary" @click="saveProfile">
                <Save :size="16" />
                <span>save</span>
              </button>
              <button @click="refreshAll">
                <RefreshCw :size="16" />
                <span>refresh</span>
              </button>
            </div>

            <label>
              <span>secret env</span>
              <textarea v-model="profileForm.secretEnvText" rows="3" placeholder="DATABASE_URL=...&#10;SQLCMDPASSWORD=..."></textarea>
            </label>
            <button class="wide" @click="unlockProfile">
              <ShieldCheck :size="16" />
              <span>unlock in memory</span>
            </button>
          </section>
        </div>
      </section>
    </div>
  </div>
</template>
