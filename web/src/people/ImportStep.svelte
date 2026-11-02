<script lang="ts">
  import FileSpreadsheet from '@lucide/svelte/icons/file-spreadsheet';
  import Upload from '@lucide/svelte/icons/upload';
  import RefreshCw from '@lucide/svelte/icons/refresh-cw';
  import Button from '../components/Button.svelte';
  import Banner from '../components/Banner.svelte';
  import Spinner from '../components/Spinner.svelte';
  import { num, plural } from '../lib/format';
  import type { PeopleList } from './list.svelte';

  let { list }: { list: PeopleList } = $props();
  let input: HTMLInputElement | undefined = $state();
  let dragging = $state(false);

  function pick(files: FileList | null | undefined) {
    const f = files?.[0];
    if (f) list.parse(f);
  }

  const preview = $derived(list.table ? list.table.rows.slice(0, 5) : []);
</script>

<div class="step">
  <div class="head">
    <h2>Import the list of people</h2>
    <p class="muted">
      Export the registrations from your sign-up or payment system as a spreadsheet file (.csv). Any column order is
      fine; you'll choose the columns next.
    </p>
  </div>

  <input
    bind:this={input}
    type="file"
    accept=".csv,.tsv,.txt,text/csv,text/tab-separated-values,text/plain"
    class="sr-only"
    tabindex="-1"
    aria-hidden="true"
    onchange={(e) => {
      pick((e.currentTarget as HTMLInputElement).files);
      (e.currentTarget as HTMLInputElement).value = '';
    }}
  />

  {#if list.table && !list.parsing}
    <div class="file">
      <span class="file-icon"><FileSpreadsheet size={26} aria-hidden={true} /></span>
      <div class="file-text">
        <p class="file-name">{list.table.fileName}</p>
        <p class="muted small">
          {plural(list.table.rows.length, 'row', 'rows')} of people · {plural(
            list.table.columns.length,
            'column',
            'columns',
          )}
        </p>
      </div>
      <Button variant="secondary" size="sm" icon={RefreshCw} onclick={() => input?.click()}
        >Choose a different file</Button
      >
    </div>

    {#if !list.table.hasHeader}
      <Banner tone="info">
        This file doesn't have a row of column names at the top, so the columns are called Column A, Column B, and so
        on.
      </Banner>
    {/if}

    <div class="preview">
      <div class="preview-head">
        <p class="preview-label">First {Math.min(5, list.table.rows.length)} of {num(list.table.rows.length)} rows</p>
        {#if list.table.columns.length > 5}
          <p class="preview-hint">Scroll sideways to see all {list.table.columns.length} columns →</p>
        {/if}
      </div>
      <!-- A scrollable region must be focusable so keyboard users can scroll it. -->
      <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
      <div class="table-wrap" role="region" aria-label="Preview of the file" tabindex="0">
        <table>
          <thead>
            <tr>
              {#each list.table.columns as col (col.index)}<th scope="col">{col.label}</th>{/each}
            </tr>
          </thead>
          <tbody>
            {#each preview as row (row.line)}
              <tr>
                {#each row.cells as cell, i (i)}<td>{cell}</td>{/each}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    </div>
  {:else}
    <div
      class="drop"
      class:dragging
      role="presentation"
      ondragenter={(e) => {
        e.preventDefault();
        dragging = true;
      }}
      ondragover={(e) => {
        e.preventDefault();
        dragging = true;
      }}
      ondragleave={() => (dragging = false)}
      ondrop={(e) => {
        e.preventDefault();
        dragging = false;
        pick(e.dataTransfer?.files);
      }}
    >
      {#if list.parsing}
        <Spinner size={32} />
        <p class="drop-title">Reading your file…</p>
      {:else}
        <span class="drop-icon"><Upload size={30} aria-hidden={true} /></span>
        <p class="drop-title">Drag your spreadsheet file (.csv) here</p>
        <p class="muted">or</p>
        <Button variant="primary" size="lg" icon={FileSpreadsheet} onclick={() => input?.click()}>Choose file</Button>
        <p class="muted small formats">Spreadsheet files ending in .csv work best. .tsv and .txt files work too.</p>
      {/if}
    </div>
  {/if}

  {#if list.fileError}
    <Banner tone="danger" title="That file didn't work" live>{list.fileError}</Banner>
  {/if}
</div>

<style>
  .step {
    display: flex;
    flex-direction: column;
    gap: 1.25rem;
  }
  .head p {
    margin-top: 0.35rem;
    max-width: 640px;
  }
  .drop {
    border: 2px dashed var(--border-strong);
    border-radius: var(--radius-lg);
    background: var(--bg-raised);
    padding: 3rem 1.5rem;
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 0.6rem;
    text-align: center;
    transition:
      border-color 0.15s ease,
      background-color 0.15s ease;
    min-height: 300px;
    justify-content: center;
  }
  .drop.dragging {
    border-color: var(--accent);
    background: var(--bg-accent-soft);
  }
  .drop-icon {
    width: 64px;
    height: 64px;
    border-radius: 50%;
    display: grid;
    place-items: center;
    background: var(--bg-accent-soft);
    color: var(--accent);
    margin-bottom: 0.4rem;
  }
  .drop-title {
    font-weight: 620;
    font-size: 1.125rem;
  }
  .formats {
    margin-top: 0.6rem;
  }
  .file {
    display: flex;
    align-items: center;
    gap: 1rem;
    padding: 1rem 1.25rem;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-raised);
  }
  .file-icon {
    width: 48px;
    height: 48px;
    border-radius: 12px;
    display: grid;
    place-items: center;
    background: var(--success-soft);
    color: var(--success);
    flex: none;
  }
  .file-text {
    flex: 1;
    min-width: 0;
  }
  .file-name {
    font-weight: 620;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .preview-head {
    display: flex;
    justify-content: space-between;
    align-items: baseline;
    gap: 1rem;
    margin-bottom: 0.5rem;
  }
  .preview-label {
    font-size: 0.875rem;
    font-weight: 600;
    color: var(--text-secondary);
  }
  .preview-hint {
    font-size: 0.8125rem;
    color: var(--text-secondary);
  }
  .table-wrap {
    overflow-x: auto;
    border: 1px solid var(--border);
    border-radius: var(--radius);
    background: var(--bg-raised);
  }
  table {
    border-collapse: collapse;
    width: 100%;
    font-size: 0.875rem;
  }
  th,
  td {
    text-align: left;
    padding: 0.6rem 0.9rem;
    white-space: nowrap;
    max-width: 260px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  th {
    background: var(--bg-inset);
    font-weight: 620;
    color: var(--text-primary);
  }
  tbody tr + tr td {
    border-top: 1px solid var(--border-subtle);
  }
</style>
