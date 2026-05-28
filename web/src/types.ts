export type Profile = {
  name: string;
  adapter: string;
  command: string;
  args: string[];
  env: Record<string, string>;
  timeoutMs: number;
  maxRows: number;
  unlocked: boolean;
};

export type RunSummary = {
  id: number;
  sessionId: string;
  profile: string;
  reason: string;
  status: string;
  startedAt: string;
  exitCode?: number;
  rowCount: number;
};

export type Annotation = {
  id: number;
  runId: number;
  rowNumber?: number;
  rowEnd?: number;
  kind: string;
  note: string;
  source: string;
  createdAt: string;
};

export type Run = {
  id: number;
  sessionId: string;
  profile: string;
  query: string;
  reason: string;
  status: string;
  startedAt: string;
  exitCode?: number;
  stdout: string;
  stderr: string;
  columns: string[];
  rowCount: number;
  parseError?: string;
  notes: Annotation[];
  highlights: Annotation[];
};

export type AgentSession = {
  id: string;
  name: string;
  createdAt: string;
  updatedAt: string;
};

export type ResultRow = {
  runId: number;
  number: number;
  cells: string[];
  highlight: boolean;
};

export type Page<T> = {
  items: T[];
  page: number;
  pageSize: number;
  total: number;
};
