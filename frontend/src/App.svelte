<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import { EventsOn, OnFileDrop } from '../wailsjs/runtime/runtime';
  import {
    OpenFile,
    GetStartupFile,
    GetStartupTheme,
    PickFile,
    ResolveTheme,
    ListThemes,
    LoadRemoteImage,
  } from '../wailsjs/go/main/App';

  // ?raw tells Vite to import the file content as a plain string — bundled
  // into the JS output at build time, no runtime fetch, nothing can 404.
  // Used as the fallback when no --theme flag was passed or ResolveTheme fails.
  import defaultDarkCSS from './themes/builtin/default-dark.css?raw';

  // ── Reactive state ──────────────────────────────────────────────────────────

  let html         = '';
  let frontMatter  = { Title: '', Date: '', Tags: [] };
  let wordCount    = 0;
  let readingMins  = 0;
  let needsMermaid = false;
  let needsMath    = false;

  // Two independent error slots so a theme error doesn't wipe a file error
  // and vice versa.
  let themeError = '';  // shown as a status bar badge; does not block file open
  let fileError  = '';  // shown as a full-page error state

  let loading    = true;
  let fileOpened = false; // true once at least one file has been successfully loaded

  // currentPath is the absolute path of the currently open file.
  // Set on every successful loadFile(); read by the file:changed handler.
  let currentPath = '';

  // ── Theme picker state ──────────────────────────────────────────────────────
  let availableThemes = [];  // populated from ListThemes() on startup
  let activeTheme    = '';   // name of the currently applied theme
  let themePickerOpen = false;

  // switchTheme fetches and applies a theme by name. Called by the dropdown
  // picker in the status bar and usable from the --theme CLI flag path.
  async function switchTheme(name) {
    if (!name || name === activeTheme) return;
    try {
      const css = await ResolveTheme(name);
      applyTheme(css);
      applyChroma(css);
      activeTheme = name;
      themeError = '';
    } catch (e) {
      themeError = String(e);
    }
  }

  function toggleThemePicker() {
    themePickerOpen = !themePickerOpen;
  }

  function selectTheme(name) {
    switchTheme(name);
    themePickerOpen = false;
  }

  function handleClickOutside(e) {
    if (themePickerOpen && !e.target.closest('.theme-picker-wrap')) {
      themePickerOpen = false;
    }
  }

  // ── Zoom state (M7) ─────────────────────────────────────────────────────────
  //
  // zoomLevel is a percentage (100 = default). Applied via CSS transform on
  // the article element so it doesn't affect layout calculations (scroll,
  // status bar position, etc.). Persisted in-session only; v1.0 can add
  // cross-session persistence via internal/state.
  let zoomLevel = 100;

  function applyZoom() {
    const article = document.querySelector('.md-body');
    if (article) {
      article.style.transform = `scale(${zoomLevel / 100})`;
      article.style.transformOrigin = 'top center';
    }
  }

  function zoomIn() {
    zoomLevel = Math.min(zoomLevel + 10, 300);
    applyZoom();
  }

  function zoomOut() {
    zoomLevel = Math.max(zoomLevel - 10, 50);
    applyZoom();
  }

  function zoomReset() {
    zoomLevel = 100;
    applyZoom();
  }

  // ── Theme injection ─────────────────────────────────────────────────────────

  // applyTheme injects CSS text into <style id="mdlight-theme"> in <head>.
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

  // applyChroma injects the syntax highlighting palette that matches the theme.
  // Separate tag from applyTheme so theme variables and chroma colors are
  // independently swappable when per-theme chroma palettes are split in v1.0.
  function applyChroma(cssText) {
    let el = document.getElementById('mdlight-chroma');
    if (!el) {
      el = document.createElement('style');
      el.id = 'mdlight-chroma';
      document.head.appendChild(el);
    }
    el.textContent = cssText;
  }

  // ── File loading ─────────────────────────────────────────────────────────────

  // loadFile is the single shared function for opening a file. Used by:
  //   - CLI path on startup
  //   - Native file picker
  //   - Drag-and-drop (OnFileDrop)
  //   - file:changed auto-reload
  //   - Conflict overlay "reload from disk"
  async function loadFile(path) {
    loading   = true;
    fileError = '';
    try {
      const payload = await OpenFile(path);
      currentPath  = path;          // track for the file:changed handler
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
      await tick();
      applyZoom();
    }
  }

  // ── Startup ──────────────────────────────────────────────────────────────────

  onMount(async () => {
    // ── 1. Resolve theme & fetch theme list ─────────────────────────────────
    // GetStartupFile, GetStartupTheme, and ListThemes are all Wails-bound Go
    // calls — they are independent and can run in parallel.
    const [startupFile, startupTheme, themes] = await Promise.all([
      GetStartupFile(),
      GetStartupTheme(),
      ListThemes().catch(() => []),
    ]);

    availableThemes = themes;

    if (startupTheme) {
      try {
        const css = await ResolveTheme(startupTheme);
        applyTheme(css);
        applyChroma(css);
        activeTheme = startupTheme;
      } catch (e) {
        // Theme resolution failed — fall back to the bundled default and keep
        // the error visible in the status bar so the user knows the flag was
        // unrecognised, without blocking the file from opening.
        themeError = String(e);
        applyTheme(defaultDarkCSS);
        applyChroma(defaultDarkCSS);
        activeTheme = 'default-dark';
      }
    } else {
      // No --theme flag: use the bundled default (no network round-trip).
      applyTheme(defaultDarkCSS);
      applyChroma(defaultDarkCSS);
      activeTheme = 'default-dark';
    }

    // ── 2. Open the file ────────────────────────────────────────────────────
    if (startupFile) {
      await loadFile(startupFile);
    } else {
      // No path on the CLI → open the native file picker.
      try {
        const picked = await PickFile();
        if (picked) {
          await loadFile(picked);
        } else {
          // User cancelled the picker — show the idle/welcome state.
          loading = false;
        }
      } catch (e) {
        fileError = String(e);
        loading   = false;
      }
    }

    // ── 3. Drag-and-drop ────────────────────────────────────────────────────
    // OnFileDrop registers a handler for files dragged onto the window.
    // The callback receives (x, y, paths[]) — we only care about paths[0]
    // since MDLight opens one file per window.
    //
    // useDropTarget = false: fire the callback on any drop anywhere in the
    // window. With true, Wails only fires when the drop lands on an element
    // that has `--wails-drop-target: drop` set as a CSS custom property —
    // and MDLight has no such elements, so true would silently swallow every
    // drop. false is correct for whole-window drop acceptance.
    OnFileDrop((_x, _y, paths) => {
      if (paths && paths.length > 0) {
        loadFile(paths[0]);
      }
    }, false);

    // ── 4. File watcher ─────────────────────────────────────────────────────
    // Register the file:changed event listener. The Go-side watcher emits
    // this after debouncing filesystem events on the open file's directory.
    //
    // Use EventsOn (not EventsOnce) — the handler must fire on every external
    // change, not just the first one.
    EventsOn('file:changed', (_changedPath) => {
      if (!currentPath) return;
      loadFile(currentPath);
    });

    // ── 5. Keyboard shortcuts (zoom) ────────────────────────────────────────
    function handleKeydown(e) {
      // Only handle zoom shortcuts when not typing in an input/textarea
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) {
        return;
      }
      if (e.ctrlKey || e.metaKey) {
        switch (e.key) {
          case '=':
          case '+':
            e.preventDefault();
            zoomIn();
            break;
          case '-':
            e.preventDefault();
            zoomOut();
            break;
          case '0':
            e.preventDefault();
            zoomReset();
            break;
        }
      }
    }
    window.addEventListener('keydown', handleKeydown);

    // ── 6. Remote image click-to-load ─────────────────────────────────────
    document.addEventListener('click', handlePlaceholderClick);
    document.addEventListener('keydown', handlePlaceholderKeydown);

    // ── 7. Theme picker: close on outside click ──────────────────────────
    document.addEventListener('click', handleClickOutside);
  });

  function handlePlaceholderClick(e) {
    const el = e.target.closest('.remote-image-placeholder');
    if (!el) return;
    e.preventDefault();
    loadRemoteImage(el);
  }

  function handlePlaceholderKeydown(e) {
    if (e.key !== 'Enter' && e.key !== ' ') return;
    const el = e.target.closest('.remote-image-placeholder');
    if (!el) return;
    e.preventDefault();
    loadRemoteImage(el);
  }

  async function loadRemoteImage(el) {
    const src = el.dataset.src;
    if (!src) return;
    try {
      const dataUri = await LoadRemoteImage(src);
      const img = document.createElement('img');
      img.alt = el.textContent.replace('[image] ', '').replace(' (click to load)', '').trim();
      img.src = dataUri;
      img.className = 'remote-image-loaded';
      el.replaceWith(img);
    } catch {
      el.textContent = el.textContent.replace('(click to load)', '(failed to load)');
    }
  }

  function handleMarkdownClick(e) {
    const link = e.target.closest('a');
    if (!link) return;
    const href = link.getAttribute('href');
    if (!href) return;
    if (href.startsWith('#')) return; // internal anchor - let webview handle
    e.preventDefault();
    window.runtime.BrowserOpenURL(href);
  }

  // Clean up event listeners on unmount
  onDestroy(() => {
    window.removeEventListener('keydown', handleKeydown);
    document.removeEventListener('click', handlePlaceholderClick);
    document.removeEventListener('keydown', handlePlaceholderKeydown);
    document.removeEventListener('click', handleClickOutside);
  });
</script>

{#if loading}
  <div class="loading">Loading…</div>

{:else if fileError}
  <div class="error">{fileError}</div>

{:else if !fileOpened}
  <!-- Idle / welcome state: picker was cancelled or no file given. -->
  <div class="loading">No file open. Drop a Markdown file here or run <code>mdlight file.md</code>.</div>

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

  <article
    class="md-body"
    data-needs-mermaid={needsMermaid}
    data-needs-math={needsMath}
    on:click={handleMarkdownClick}
  >
    {@html html}
  </article>

  <footer class="status-bar">
    <span>{wordCount} words</span>
    <span>{readingMins} min read</span>
    {#if needsMermaid}<span class="flag">mermaid</span>{/if}
    {#if needsMath}<span class="flag">math</span>{/if}
    {#if availableThemes.length > 0}
      <div class="theme-picker-wrap">
        <button class="theme-picker-btn" on:click={toggleThemePicker}>
          {activeTheme}
        </button>
        {#if themePickerOpen}
          <div class="theme-picker-menu" role="listbox">
            {#each availableThemes as theme}
              <button
                class="theme-picker-item"
                class:selected={theme.Name === activeTheme}
                on:click={() => selectTheme(theme.Name)}
                role="option"
                aria-selected={theme.Name === activeTheme}
              >
                {theme.Name}
              </button>
            {/each}
          </div>
        {/if}
      </div>
    {/if}
    {#if themeError}
      <span class="flag" style="color: var(--md-error-fg);" title={themeError}>
        theme error
      </span>
    {/if}
    {#if zoomLevel !== 100}
      <span class="zoom-reset" title="Click to reset zoom" on:click={zoomReset} on:keydown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); zoomReset(); } }} tabindex="0" role="button">
        {zoomLevel}%
      </span>
    {/if}
  </footer>
{/if}

<style>
  .theme-picker-wrap {
    position: relative;
    display: inline-block;
  }

  .theme-picker-btn {
    background: var(--md-statusbar-bg);
    color: var(--md-link-color);
    border: 1px solid var(--md-hr-color);
    border-radius: 3px;
    font-family: var(--md-font-mono);
    font-size: 0.68rem;
    padding: 0.1em 0.5em;
    cursor: pointer;
    white-space: nowrap;
    line-height: 1.5;
  }

  .theme-picker-btn:hover,
  .theme-picker-btn:focus {
    border-color: var(--md-link-color);
    outline: none;
  }

  .theme-picker-menu {
    position: absolute;
    bottom: 100%;
    right: 0;
    margin-bottom: 4px;
    background: var(--md-bg);
    border: 1px solid var(--md-hr-color);
    border-radius: 4px;
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.35);
    z-index: 300;
    min-width: 100%;
    white-space: nowrap;
    overflow: hidden;
  }

  .theme-picker-item {
    display: block;
    width: 100%;
    padding: 0.3em 0.7em;
    background: transparent;
    color: var(--md-fg);
    border: none;
    font-family: var(--md-font-mono);
    font-size: 0.68rem;
    text-align: left;
    cursor: pointer;
    line-height: 1.5;
  }

  .theme-picker-item:hover {
    background: var(--md-link-color);
    color: var(--md-bg);
  }

  .theme-picker-item.selected {
    color: var(--md-link-color);
    font-weight: 600;
  }

  .theme-picker-item.selected:hover {
    color: var(--md-bg);
  }
</style>