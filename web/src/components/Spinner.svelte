<script lang="ts">
  let { size = 20, label }: { size?: number; label?: string } = $props();
</script>

<span class="spinner" style:width="{size}px" style:height="{size}px" role={label ? 'status' : undefined}>
  <svg viewBox="0 0 24 24" fill="none" aria-hidden="true">
    <circle cx="12" cy="12" r="9.5" stroke="currentColor" stroke-opacity="0.2" stroke-width="2.5" />
    <path d="M21.5 12a9.5 9.5 0 0 0-9.5-9.5" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" />
  </svg>
  {#if label}<span class="sr-only">{label}</span>{/if}
</span>

<style>
  .spinner {
    display: inline-flex;
    flex: none;
  }
  svg {
    width: 100%;
    height: 100%;
    animation: spin 0.8s linear infinite;
  }
  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }
</style>
