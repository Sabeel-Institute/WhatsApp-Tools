// The shared "who's on the list" state behind the Import, Match and Review
// steps. Any workflow that starts from a spreadsheet reuses it.
import { api, ApiError } from '../lib/api';
import { app } from '../lib/state.svelte';
import type { Job, NameMode, ParseResult, Person, RememberedMapping, Review, Suggestion, Table } from '../lib/types';

export interface Mapping {
  phone: number;
  nameMode: NameMode;
  name: number;
  first: number;
  last: number;
}

export type Field = 'phone' | 'name' | 'first' | 'last';

export class PeopleList {
  table = $state<Table | null>(null);
  parsing = $state(false);
  fileError = $state<string | null>(null);

  mapping = $state<Mapping>({ phone: -1, nameMode: 'none', name: -1, first: -1, last: -1 });
  /** Fields the app picked; cleared when the user changes them. */
  suggested = $state<Record<Field, boolean>>({ phone: false, name: false, first: false, last: false });
  remembered = $state(false);
  country = $state('US');

  /** Phone numbers edited in the review, by spreadsheet row. */
  edits = $state<Record<number, string>>({});
  /** Rows the user unticked, by spreadsheet row. */
  excluded = $state<Record<number, boolean>>({});

  review = $state<Review | null>(null);
  reviewing = $state(false);
  reviewError = $state<string | null>(null);
  checkJobId = $state<string | null>(null);
  checkError = $state<string | null>(null);

  async loadSettings() {
    try {
      const s = await api.get<{ defaultCountry: string }>('/api/settings');
      this.country = s.defaultCountry;
    } catch {
      /* keep the default */
    }
  }

  async parse(file: File) {
    this.parsing = true;
    this.fileError = null;
    try {
      const res = await api.upload<ParseResult>(`/api/sheets/parse?name=${encodeURIComponent(file.name)}`, file);
      if (res.table.rows.length === 0) {
        this.fileError =
          'This file has a header row but no people in it. Please check that you exported the right list.';
        return;
      }
      this.table = res.table;
      this.applySuggestion(res.suggestion, res.remembered);
      this.edits = {};
      this.excluded = {};
      this.review = null;
    } catch (e) {
      this.fileError = (e as ApiError).message;
    } finally {
      this.parsing = false;
    }
  }

  clearFile() {
    this.table = null;
    this.review = null;
    this.fileError = null;
  }

  private applySuggestion(s: Suggestion, remembered: RememberedMapping | null) {
    const cols = this.table!.columns;
    const byHeader = (h?: string) => (h ? cols.findIndex((c) => c.header === h) : -1);
    if (remembered && byHeader(remembered.phone) >= 0) {
      this.mapping = {
        phone: byHeader(remembered.phone),
        nameMode: remembered.nameMode,
        name: byHeader(remembered.name),
        first: byHeader(remembered.first),
        last: byHeader(remembered.last),
      };
      if (
        (this.mapping.nameMode === 'full' && this.mapping.name < 0) ||
        (this.mapping.nameMode === 'split' && (this.mapping.first < 0 || this.mapping.last < 0))
      ) {
        this.mapping.nameMode = 'none';
      }
      this.remembered = true;
      this.suggested = { phone: false, name: false, first: false, last: false };
      return;
    }
    this.remembered = false;
    this.mapping = { phone: s.phone, nameMode: s.nameMode, name: s.name, first: s.first, last: s.last };
    this.suggested = {
      phone: s.phone >= 0,
      name: s.nameMode === 'full' && s.name >= 0,
      first: s.nameMode === 'split' && s.first >= 0,
      last: s.nameMode === 'split' && s.last >= 0,
    };
  }

  get mappingComplete(): boolean {
    const m = this.mapping;
    if (m.phone < 0) return false;
    if (m.nameMode === 'full') return m.name >= 0;
    if (m.nameMode === 'split') return m.first >= 0 && m.last >= 0;
    return true;
  }

  /** Saves the column choices for files with the same headers, and the country. */
  async rememberChoices() {
    const t = this.table;
    if (!t) return;
    const m = this.mapping;
    const header = (i: number) => (i >= 0 ? t.columns[i].header : undefined);
    api.put('/api/settings/country', { country: this.country }).catch(() => {});
    if (!t.hasHeader) return;
    api
      .put('/api/settings/mapping', {
        headers: t.columns.map((c) => c.header),
        mapping: {
          phone: header(m.phone),
          nameMode: m.nameMode,
          name: m.nameMode === 'full' ? header(m.name) : undefined,
          first: m.nameMode === 'split' ? header(m.first) : undefined,
          last: m.nameMode === 'split' ? header(m.last) : undefined,
        },
      })
      .catch(() => {});
  }

  nameOf(cells: string[]): string {
    const m = this.mapping;
    if (m.nameMode === 'full' && m.name >= 0) return cells[m.name] ?? '';
    if (m.nameMode === 'split') {
      return [cells[m.first] ?? '', cells[m.last] ?? ''].filter(Boolean).join(' ');
    }
    return '';
  }

  async runReview() {
    const t = this.table;
    if (!t || this.mapping.phone < 0) return;
    this.reviewing = true;
    this.reviewError = null;
    try {
      const rows = t.rows.map((r) => ({
        row: r.line,
        name: this.nameOf(r.cells),
        phone: this.edits[r.line] ?? r.cells[this.mapping.phone] ?? '',
      }));
      this.review = await api.post<Review>('/api/people/review', { rows, country: this.country });
    } catch (e) {
      this.reviewError = (e as ApiError).message;
    } finally {
      this.reviewing = false;
    }
  }

  /** Starts looking up any usable numbers WhatsApp hasn't confirmed yet. */
  async checkWhatsApp() {
    const r = this.review;
    if (!r || !app.connected) return;
    const phones = r.people.filter((p) => usable(p) && p.onWhatsApp === null).map((p) => p.phone!);
    if (phones.length === 0) return;
    this.checkError = null;
    try {
      const { job } = await api.post<{ job: Job | null }>('/api/people/check', { phones });
      if (job) {
        app.setJob(job);
        this.checkJobId = job.id;
      } else {
        await this.runReview();
      }
    } catch (e) {
      this.checkError = (e as ApiError).message;
    }
  }

  get checkJob(): Job | null {
    return this.checkJobId ? (app.jobs[this.checkJobId] ?? null) : null;
  }

  isIncluded(p: Person): boolean {
    return usable(p) && p.onWhatsApp !== false && !this.excluded[p.row];
  }

  /** The people who will be added. */
  get selected(): Person[] {
    return this.review?.people.filter((p) => this.isIncluded(p)) ?? [];
  }

  async editPhone(row: number, value: string) {
    this.edits[row] = value;
    await this.runReview();
    const p = this.review?.people.find((x) => x.row === row);
    if (p && usable(p) && p.onWhatsApp === null) await this.checkWhatsApp();
  }
}

export const usable = (p: Person) => p.status === 'ready' || p.status === 'fixed';
