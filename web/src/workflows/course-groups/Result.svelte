<script lang="ts">
  import type { Run } from '../../lib/types';
  let { run }: { run: Run } = $props();
</script>

<p>{run.title}</p>
