import type { Component } from 'svelte';
import CircleCheck from '@lucide/svelte/icons/circle-check';
import UserCheck from '@lucide/svelte/icons/user-check';
import Link2 from '@lucide/svelte/icons/link-2';
import MessageCircleOff from '@lucide/svelte/icons/message-circle-off';
import CircleAlert from '@lucide/svelte/icons/circle-alert';
import Clock from '@lucide/svelte/icons/clock';
import Send from '@lucide/svelte/icons/send';
import UsersRound from '@lucide/svelte/icons/users-round';

/** coursegroups.Person */
export interface CGPerson {
  row: number;
  name: string;
  phone: string;
  display: string;
  outcome: string;
  note?: string;
  inviteStatus?: '' | 'sent' | 'failed';
}

/** coursegroups.Detail */
export interface CGDetail {
  mode: 'create' | 'add';
  communityId: string;
  communityName: string;
  groupId?: string;
  groupName: string;
  description?: string;
  settings: { onlyAdminsSend: boolean; onlyAdminsEdit: boolean; approvalRequired: boolean };
  inviteLink?: string;
  inviteMessage?: string;
  warnings?: string[];
  people: CGPerson[];
}

type Tone = 'success' | 'warning' | 'danger' | 'info' | 'neutral' | 'accent';
type Icon = Component<{ size?: number; 'aria-hidden'?: boolean }>;

export const outcomes: Record<string, { text: string; tone: Tone; icon: Icon; group: string }> = {
  added: { text: 'Added', tone: 'success', icon: CircleCheck, group: 'added' },
  already_member: { text: 'Already in the group', tone: 'success', icon: UserCheck, group: 'added' },
  privacy: { text: 'Needs an invite', tone: 'warning', icon: Link2, group: 'invite' },
  recently_left: { text: 'Needs an invite (left recently)', tone: 'warning', icon: Link2, group: 'invite' },
  not_on_whatsapp: { text: 'Not on WhatsApp', tone: 'neutral', icon: MessageCircleOff, group: 'none' },
  failed: { text: "Couldn't add", tone: 'danger', icon: CircleAlert, group: 'failed' },
  pending: { text: 'Not added yet', tone: 'neutral', icon: Clock, group: 'pending' },
  // feed-only outcomes
  created: { text: 'Group created', tone: 'accent', icon: UsersRound, group: '' },
  invite_sent: { text: 'Invite sent', tone: 'success', icon: Send, group: '' },
  invite_failed: { text: "Invite couldn't be sent", tone: 'danger', icon: CircleAlert, group: '' },
};

export const outcomeOf = (o: string) => outcomes[o] ?? outcomes.failed;
