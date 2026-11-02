// The web half of the workflow registry (the Go half is
// internal/workflows). Each workflow ID maps to its wizard and to the
// component that shows one of its saved runs.
import type { Component } from 'svelte';
import UsersRound from '@lucide/svelte/icons/users-round';
import UserPlus from '@lucide/svelte/icons/user-plus';
import Workflow from '@lucide/svelte/icons/workflow';
import CourseGroupsWizard from './course-groups/Wizard.svelte';
import CourseGroupsResult from './course-groups/Result.svelte';
import type { Run } from '../lib/types';

export interface WorkflowUI {
  wizard: Component<{ action: string }>;
  result: Component<{ run: Run }>;
}

const workflows: Record<string, WorkflowUI> = {
  'course-groups': { wizard: CourseGroupsWizard, result: CourseGroupsResult as Component<{ run: Run }> },
};

export function workflowScreen(id: string | undefined): Component<{ action: string }> | null {
  return (id && workflows[id]?.wizard) || null;
}

export function resultScreen(id: string): Component<{ run: Run }> | null {
  return workflows[id]?.result ?? null;
}

/** Icons the server may name for home-screen actions. */
const icons: Record<string, Component<{ size?: number; 'aria-hidden'?: boolean }>> = {
  'users-round': UsersRound,
  'user-plus': UserPlus,
};

export function actionIcon(name: string): Component<{ size?: number; 'aria-hidden'?: boolean }> {
  return icons[name] ?? Workflow;
}
