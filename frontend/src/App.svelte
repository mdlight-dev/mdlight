<script>
  import { onMount } from 'svelte';
  import { OpenFile } from '../wailsjs/go/main/App';

  // DocumentPayload fields — mirrors the Go struct.
  let html = '';
  let frontMatter = { Title: '', Date: '', Tags: [] };
  let wordCount = 0;
  let readingMins = 0;
  let needsMermaid = false;
  let needsMath = false;
  let error = '';
  let loading = true;

  onMount(async () => {
    try {
      // Milestone 2: pass empty string so Go uses the hardcoded test path.
      // Milestone 4 will pass the real path from CLI args via a startup event.
      const payload = await OpenFile('');

      html        = payload.HTML;
      frontMatter = payload.FrontMatter;
      wordCount   = payload.WordCount;
      readingMins = payload.ReadingMins;
      needsMermaid = payload.NeedsMermaid;
      needsMath    = payload.NeedsMath;
    } catch (e) {
      error = String(e);
    } finally {
      loading = false;
    }
  });
</script>

{#if loading}
  <div class="loading">Loading…</div>
{:else if error}
  <div class="error">{error}</div>
{:else}
  <!-- Front matter card (v1.0 will style this properly) -->
  {#if frontMatter.Title}
    <div class="frontmatter-card">
      <h1 class="fm-title">{frontMatter.Title}</h1>
      {#if frontMatter.Date}<span class="fm-date">{frontMatter.Date}</span>{/if}
      {#if frontMatter.Tags?.length}
        <div class="fm-tags">
          {#each frontMatter.Tags as tag}<span class="fm-tag">{tag}</span>{/each}
        </div>
      {/if}
    </div>
  {/if}

  <!-- Rendered Markdown body -->
  <!-- svelte-ignore a11y-no-static-element-interactions -->
  <article class="md-body" data-needs-mermaid={needsMermaid} data-needs-math={needsMath}>
    {@html html}
  </article>

  <!-- Status bar -->
  <footer class="status-bar">
    <span>{wordCount} words</span>
    <span>{readingMins} min read</span>
    {#if needsMermaid}<span class="flag">mermaid</span>{/if}
    {#if needsMath}<span class="flag">math</span>{/if}
  </footer>
{/if}

<style>
  /* Temporary structural styles for milestone 2 only.
     These will be replaced wholesale by style.css + the default theme in milestone 3. */

  .loading, .error {
    padding: 2rem;
    font-family: sans-serif;
    color: #666;
  }
  .error { color: #c00; }

  .frontmatter-card {
    padding: 1rem 2rem;
    border-bottom: 1px solid #eee;
    font-family: sans-serif;
  }
  .fm-title  { margin: 0 0 0.25rem; font-size: 1.1rem; }
  .fm-date   { font-size: 0.85rem; color: #888; }
  .fm-tags   { margin-top: 0.5rem; }
  .fm-tag    { display: inline-block; margin-right: 0.4rem; padding: 0.1rem 0.4rem;
               background: #f0f0f0; border-radius: 3px; font-size: 0.8rem; }

  .md-body {
    max-width: 720px;
    margin: 2rem auto;
    padding: 0 2rem;
    font-family: serif;
    line-height: 1.7;
  }

  .status-bar {
    position: fixed;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 0.4rem 1rem;
    background: #f8f8f8;
    border-top: 1px solid #eee;
    font-family: sans-serif;
    font-size: 0.8rem;
    color: #888;
    display: flex;
    gap: 1rem;
  }
  .flag {
    background: #e8f0fe;
    padding: 0 0.4rem;
    border-radius: 3px;
    color: #3c6;
  }
</style>