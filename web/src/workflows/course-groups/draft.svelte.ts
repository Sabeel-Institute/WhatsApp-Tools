// Everything the user has entered in a Course groups wizard. It lives for
// the whole session, so leaving the wizard (or going Back) never loses work.
import { api, ApiError } from '../../lib/api';
import { app } from '../../lib/state.svelte';
import { PeopleList } from '../../people/list.svelte';
import type { Community } from '../../lib/types';

export type Mode = 'create' | 'add';

export interface GroupSettings {
  onlyAdminsSend: boolean;
  onlyAdminsEdit: boolean;
  approvalRequired: boolean;
}

export class Draft {
  mode: Mode;
  step = $state(0);
  reached = $state(0);
  list = new PeopleList();

  communities = $state<Community[] | null>(null);
  loadingCommunities = $state(false);
  communitiesError = $state<string | null>(null);

  communityId = $state('');
  groupId = $state('');
  groupName = $state('');
  description = $state('');
  settings = $state<GroupSettings>({ onlyAdminsSend: false, onlyAdminsEdit: true, approvalRequired: true });

  starting = $state(false);
  startError = $state<string | null>(null);

  constructor(mode: Mode) {
    this.mode = mode;
    this.list.loadSettings();
  }

  go(step: number) {
    this.step = step;
    this.reached = Math.max(this.reached, step);
    window.scrollTo({ top: 0 });
  }

  async loadCommunities(force = false) {
    if (this.loadingCommunities || (this.communities && !force)) return;
    this.loadingCommunities = true;
    this.communitiesError = null;
    try {
      this.communities = await api.get<Community[]>('/api/wa/communities');
      this.pickDefaults();
    } catch (e) {
      this.communitiesError = (e as ApiError).message;
    } finally {
      this.loadingCommunities = false;
    }
  }

  /** Communities the account can create groups in. */
  get adminCommunities(): Community[] {
    return (this.communities ?? []).filter((c) => c.isAdmin && c.id !== 'standalone');
  }

  /** Communities with at least one group the account can add people to. */
  get addableCommunities(): Community[] {
    return (this.communities ?? []).filter((c) => c.groups.some((g) => g.isAdmin));
  }

  private pickDefaults() {
    const options = this.mode === 'create' ? this.adminCommunities : this.addableCommunities;
    if (!options.some((c) => c.id === this.communityId)) this.communityId = options.length === 1 ? options[0].id : '';
    if (this.mode === 'add') {
      const groups = this.community?.groups.filter((g) => g.isAdmin) ?? [];
      if (!groups.some((g) => g.id === this.groupId)) this.groupId = groups.length === 1 ? groups[0].id : '';
    }
  }

  get community(): Community | undefined {
    return this.communities?.find((c) => c.id === this.communityId);
  }

  get group() {
    return this.community?.groups.find((g) => g.id === this.groupId);
  }

  get nameError(): string | null {
    const n = this.groupName.trim().length;
    if (n === 0) return null;
    const max = app.info?.maxNameLength ?? 25;
    return n > max ? `Group names can be at most ${max} characters.` : null;
  }

  /** Members after the run, including the connected account. */
  get finalSize(): number {
    const adding = this.list.selected.length;
    return this.mode === 'create' ? adding + 1 : (this.group?.size ?? 0) + adding;
  }

  get overLimit(): boolean {
    return this.finalSize > (app.info?.maxGroupSize ?? 1024);
  }

  get detailsComplete(): boolean {
    if (this.overLimit) return false;
    if (this.mode === 'create') {
      const n = this.groupName.trim().length;
      return (
        !!this.community &&
        n > 0 &&
        n <= (app.info?.maxNameLength ?? 25) &&
        this.description.length <= (app.info?.maxDescLength ?? 2048)
      );
    }
    return !!this.group;
  }

  /** Starts the run and returns its ID. */
  async start(): Promise<string | null> {
    this.starting = true;
    this.startError = null;
    try {
      const people = this.list.selected.map((p) => ({ row: p.row, name: p.name, phone: p.phone, display: p.display }));
      const res = await api.post<{ runId: string; job: import('../../lib/types').Job }>(
        '/api/workflows/course-groups/run',
        {
          mode: this.mode,
          communityId: this.communityId,
          communityName: this.community?.name ?? '',
          groupId: this.mode === 'add' ? this.groupId : '',
          groupName: this.mode === 'create' ? this.groupName.trim() : (this.group?.name ?? ''),
          description: this.mode === 'create' ? this.description.trim() : '',
          settings: this.settings,
          people,
        },
      );
      app.setJob(res.job);
      return res.runId;
    } catch (e) {
      this.startError = (e as ApiError).message;
      return null;
    } finally {
      this.starting = false;
    }
  }
}

const drafts: Partial<Record<Mode, Draft>> = {};

export function draftFor(mode: Mode): Draft {
  return (drafts[mode] ??= new Draft(mode));
}

export function resetDraft(mode: Mode) {
  delete drafts[mode];
}
