<script>
  import { onMount } from 'svelte';
  import { OpenFile } from '../wailsjs/go/main/App';

  // ?raw tells Vite to import the file content as a plain string.
  // This works in both `wails dev` and `wails build` (the string is bundled
  // into the JS output, so no runtime fetch is needed and nothing can 404).
  // Milestone 4 replaces this import with a ResolveTheme() Go binding call
  // so user themes and the --theme flag work. For milestone 3, a static
  // import is exactly right: one theme, zero network round-trips.
  import defaultDarkCSS from './themes/builtin/default-dark.css?raw';

  let html = '';
  let frontMatter = { Title: '', Date: '', Tags: [] };
  let wordCount = 0;
  let readingMins = 0;
  let needsMermaid = false;
  let needsMath = false;
  let error = '';
  let loading = true;

  // Inject the active theme's CSS text into <style id="mdlight-theme">.
  // Called on startup and on every theme switch (milestone 4+).
  function applyTheme(cssText) {
    let el = document.getElementById('mdlight-theme');
    if (!el) {
      el = document.createElement('style');
      el.id = 'mdlight-theme';
      document.head.appendChild(el);
    }
    el.textContent = cssText;
  }

  // Inject the chroma palette that matches the active theme.
  // Separate tag keeps theme variables and syntax colors independently
  // swappable when milestone 4 wires up per-theme chroma palettes.
  function applyChroma(cssText) {
    let el = document.getElementById('mdlight-chroma');
    if (!el) {
      el = document.createElement('style');
      el.id = 'mdlight-chroma';
      document.head.appendChild(el);
    }
    el.textContent = cssText;
  }

  onMount(async () => {
    try {
      // Both style tags get the same source in milestone 3: default-dark.css
      // contains the :root {} block and the .chroma classes in one file.
      // Milestone 4's ResolveTheme() will return just the :root block; at
      // that point the chroma palette ships as a separate file per theme.
      applyTheme(defaultDarkCSS);
      applyChroma(defaultDarkCSS);

      // Load the document.
      // Milestone 2: pass empty string so Go uses the hardcoded test path.
      // Milestone 4 will pass the real path from CLI args via a startup event.
      const payload = await OpenFile('');

      html         = payload.HTML;
      frontMatter  = payload.FrontMatter;
      wordCount    = payload.WordCount;
      readingMins  = payload.ReadingMins;
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

  <article class="md-body" data-needs-mermaid={needsMermaid} data-needs-math={needsMath}>
    {@html html}
  </article>

  <footer class="status-bar">
    <span>{wordCount} words</span>
    <span>{readingMins} min read</span>
    {#if needsMermaid}<span class="flag">mermaid</span>{/if}
    {#if needsMath}<span class="flag">math</span>{/if}
  </footer>
{/if}

<style>
  /*
    This block contains only rules that need Svelte's component scoping —
    i.e. rules that would bleed into rendered Markdown if written globally.
    Everything else lives in style.css (structure) and the theme file (skin).

    As of milestone 3 there are no such rules: all structural classes
    (.md-body, .frontmatter-card, .status-bar, .loading, .error) are
    intentionally global so the theme variables reach them.

    This block is intentionally empty. It exists as a placeholder so the
    pattern is clear for any future component-scoped rules.
  */
</style>