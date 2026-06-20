<script>
  import { onMount } from 'svelte';
  import { OpenFile, GetStartupFile, GetStartupTheme, PickFile, ResolveTheme } from '../wailsjs/go/main/App';
  import { OnFileDrop } from '../wailsjs/runtime';

  // Static import of the default theme — bundled at build time by Vite so
  // there is no runtime fetch and nothing can 404. Used as the fallback when
  // no --theme flag is given, and also as the value shown before the Go
  // binding round-trip completes (zero flash of unstyled content).
  import defaultDarkCSS from './themes/builtin/default-dark.css?raw';

  let html = '';
  let frontMatter = { Title: '', Date: '', Tags: [] };
  let wordCount = 0;
  let readingMins = 0;
  let needsMermaid = false;
  let needsMath = false;
  // Two independent error slots so a successful file open doesn't clobber
  // a theme error, and a theme error doesn't mask a file error.
  let themeError = ''; // set when --theme name is not found
  let fileError  = ''; // set when OpenFile / PickFile fails
  let loading = true;
  // Tracks whether a file has been successfully opened. Distinct from html
  // being non-empty — an empty file or a front-matter-only file produces
  // html === '' but is still a valid open document that should render
  // (showing word count 0, metadata card if present, etc.).
  let fileOpened = false;

  // ── Theme injection ────────────────────────────────────────────────────

  // Inject the active theme's :root {} block + any extra selectors into
  // <style id="mdlight-theme">. Called on startup and on every theme switch.
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
  // Separate <style> tag keeps theme variables and syntax colors independently
  // swappable when per-theme chroma palette files are split in v1.0.
  // For now (M3/M4) the same combined file feeds both tags — harmless.
  function applyChroma(cssText) {
    let el = document.getElementById('mdlight-chroma');
    if (!el) {
      el = document.createElement('style');
      el.id = 'mdlight-chroma';
      document.head.appendChild(el);
    }
    el.textContent = cssText;
  }

  // ── Document loading ───────────────────────────────────────────────────

  // loadFile is the single code path for opening a document regardless of
  // how the path was obtained (CLI arg, file picker, or drag-and-drop).
  async function loadFile(path) {
    loading = true;
    fileError = '';
    try {
      const payload = await OpenFile(path);
      html         = payload.HTML;
      frontMatter  = payload.FrontMatter;
      wordCount    = payload.WordCount;
      readingMins  = payload.ReadingMins;
      needsMermaid = payload.NeedsMermaid;
      needsMath    = payload.NeedsMath;
      fileOpened   = true;
    } catch (e) {
      fileError = String(e);
    } finally {
      loading = false;
    }
  }

  // ── Startup ────────────────────────────────────────────────────────────

  onMount(async () => {
    try {
      // Apply the default theme immediately — before any network round-trip —
      // so there is no flash of unstyled content while the Go bindings resolve.
      applyTheme(defaultDarkCSS);
      applyChroma(defaultDarkCSS);

      // Option B from HANDOFF_milestone4: call bindings to get the CLI values.
      // This is race-free: bindings are always available by the time onMount
      // runs, unlike EventsOn which can fire before the listener is registered.
      const [filePath, themeName] = await Promise.all([
        GetStartupFile(),
        GetStartupTheme(),
      ]);

      // Resolve theme: if --theme was passed, fetch it from Go (which follows
      // the XDG → builtin → error resolution order). Otherwise keep the static
      // import already applied above.
      if (themeName) {
        try {
          const css = await ResolveTheme(themeName);
          applyTheme(css);
          applyChroma(css);
        } catch (themeErr) {
          // Bad --theme value: surface the Go error (which includes available
          // theme names). Default theme stays applied. Stored in themeError
          // separately so a successful file open doesn't clear it.
          themeError = String(themeErr);
        }
      }

      // Open the file: CLI path → loadFile directly.
      // No path → open the native file picker, then loadFile on the choice.
      // Picker cancelled (empty string) → show an idle state, no error.
      if (filePath) {
        await loadFile(filePath);
      } else {
        const chosen = await PickFile();
        if (chosen) {
          await loadFile(chosen);
        } else {
          // User cancelled the picker — show an empty idle state.
          loading = false;
        }
      }
    } catch (e) {
      fileError = String(e);
      loading = false;
    }

    // ── Drag-and-drop ────────────────────────────────────────────────
    // Wails v2: OnFileDrop(callback, useDropTarget).
    // The callback receives (x, y, paths[]) — we take the first path only
    // (dropping multiple files opens the first one; multi-file support is
    // out of scope for v0.1).
    OnFileDrop((_x, _y, paths) => {
      if (paths && paths.length > 0) {
        loadFile(paths[0]);
      }
    }, true);
  });
</script>

{#if loading}
  <div class="loading">Loading…</div>
{:else if fileError}
  <div class="error">{fileError}</div>
{:else if !fileOpened}
  <!-- Picker was cancelled or no file passed; idle state -->
  <div class="loading">Open a Markdown file to begin.</div>
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
    {#if themeError}<span class="flag theme-error" title={themeError}>theme not found</span>{/if}
  </footer>
{/if}

<style>
  /*
    This block contains only rules that need Svelte's component scoping.
    All structural and visual rules live in style.css and the theme file.
  */

  /* theme-error flag in the status bar — scoped so it doesn't bleed into
     rendered Markdown content which might also have .flag elements. */
  .theme-error {
    color: var(--md-error-fg, #e08a8a);
    border-color: var(--md-error-fg, #e08a8a);
    cursor: help;
  }
</style>