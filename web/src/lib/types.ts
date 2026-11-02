// Shapes of the JSON the Go server sends. Keep in sync with the Go structs
// named in each comment.

/** whatsapp.State */
export type WAState =
  | 'starting'
  | 'unlinked'
  | 'linking'
  | 'connecting'
  | 'connected'
  | 'logged_out'
  | 'outdated'
  | 'banned'
  | 'replaced';

export interface Account {
  name: string;
  phone: string;
}

/** whatsapp.Status */
export interface WAStatus {
  state: WAState;
  account?: Account;
  detail?: string;
  since: string;
}

/** server.loginEventView */
export interface LoginEvent {
  kind: 'qr' | 'pair_code' | 'success' | 'expired' | 'error' | 'canceled';
  qrSvg?: string;
  code?: string;
  expiresIn?: number;
  reason?: string;
  account?: Account;
}

/** workflows.Action / workflows.Info */
export interface WorkflowAction {
  id: string;
  label: string;
  description: string;
  primary: boolean;
  icon: string;
}

export interface WorkflowInfo {
  id: string;
  title: string;
  description: string;
  actions: WorkflowAction[];
}

export interface AppInfo {
  name: string;
  version: string;
  demo: boolean;
  dataDir: string;
  workflows: WorkflowInfo[];
  maxGroupSize: number;
  maxNameLength: number;
  maxDescLength: number;
  pacing: {
    addBatchSize: number;
    batchPauseMinSeconds: number;
    batchPauseMaxSeconds: number;
    invitePauseMinSeconds: number;
    invitePauseMaxSeconds: number;
  };
}

/** jobs.Snapshot */
export type JobState = 'running' | 'paused' | 'waiting' | 'stopping' | 'completed' | 'stopped' | 'failed';

export interface JobEntry {
  at: string;
  title: string;
  detail?: string;
  outcome: string;
}

export interface Job {
  id: string;
  workflow: string;
  kind: string;
  title: string;
  runId?: string;
  state: JobState;
  phase: string;
  message?: string;
  done: number;
  total: number;
  counts: Record<string, number>;
  etaSeconds: number;
  startedAt: string;
  finishedAt?: string;
  recent: JobEntry[];
  error?: string;
}

export const jobFinished = (s: JobState) => s === 'completed' || s === 'stopped' || s === 'failed';

/** runs.Run */
export type RunStatus = 'running' | 'completed' | 'stopped' | 'failed' | 'interrupted';

export interface Run<D = unknown> {
  id: string;
  workflow: string;
  kind: string;
  title: string;
  subtitle?: string;
  status: RunStatus;
  startedAt: string;
  finishedAt?: string;
  headline: string;
  counts: Record<string, number>;
  error?: string;
  detail?: D;
}

/** csvfile.Column / Row / Table / Suggestion */
export interface Column {
  index: number;
  label: string;
  header: string;
  samples: string[];
}

export interface SheetRow {
  line: number;
  cells: string[];
}

export interface Table {
  fileName: string;
  encoding: string;
  delimiter: string;
  hasHeader: boolean;
  columns: Column[];
  rows: SheetRow[];
}

export type NameMode = 'none' | 'full' | 'split';

export interface Suggestion {
  phone: number;
  nameMode: NameMode;
  name: number;
  first: number;
  last: number;
}

/** settings.Mapping (by header text) */
export interface RememberedMapping {
  phone: string;
  nameMode: NameMode;
  name?: string;
  first?: string;
  last?: string;
}

export interface ParseResult {
  table: Table;
  suggestion: Suggestion;
  remembered: RememberedMapping | null;
}

/** people.Person / Counts / Review */
export type PersonStatus = 'ready' | 'fixed' | 'attention' | 'duplicate' | 'self' | 'not_on_whatsapp';

export interface Person {
  row: number;
  name: string;
  raw: string;
  phone?: string;
  display?: string;
  status: PersonStatus;
  note?: string;
  duplicateOf?: number;
  alsoRows?: number[];
  onWhatsApp: boolean | null;
}

export interface ReviewCounts {
  total: number;
  ready: number;
  fixed: number;
  attention: number;
  duplicates: number;
  notOnWhatsApp: number;
  unchecked: number;
}

export interface Review {
  people: Person[];
  counts: ReviewCounts;
}

export interface Country {
  code: string;
  name: string;
  dial: string;
}

/** whatsapp.Community / Group */
export interface Group {
  id: string;
  name: string;
  size: number;
  isAdmin: boolean;
  communityId: string;
}

export interface Community {
  id: string;
  name: string;
  isAdmin: boolean;
  groups: Group[];
}
