<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import { EventsOn, OnFileDrop } from '../wailsjs/runtime/runtime';
  import {
    OpenFile,
    GetStartupFile,
    GetStartupTheme,
    PickFile,
    ResolveTheme,
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

  // ── Edit / conflict state (M5: wired now; activated in v1.0 edit mode) ────
  //
  // dirty becomes true when the user has unsaved edits in split-pane edit mode
  // (v1.0). For v0.1 it is always false — there is no edit mode yet — so the
  // conflict overlay is structurally wired but never actually shown.
  let dirty        = false;
  let showConflict = false;

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

  // ── Conflict overlay handlers ────────────────────────────────────────────────

  // keepMine dismisses the overlay without reloading. The user's unsaved edits
  // remain in place. The on-disk version is silently ignored.
  function keepMine() {
    showConflict = false;
  }

  // reloadFromDisk dismisses the overlay, clears dirty state, and reloads the
  // file from disk — discarding any unsaved edits.
  async function reloadFromDisk() {
    showConflict = false;
    dirty        = false;
    await loadFile(currentPath);
  }

  // ── Startup ──────────────────────────────────────────────────────────────────

  onMount(async () => {
    // ── 1. Resolve theme ────────────────────────────────────────────────────
    // Call GetStartupFile and GetStartupTheme in parallel — both are simple
    // struct field reads on the Go side, no I/O, so there's no ordering
    // dependency between them.
    const [startupFile, startupTheme] = await Promise.all([
      GetStartupFile(),
      GetStartupTheme(),
    ]);

    if (startupTheme) {
      try {
        const css = await ResolveTheme(startupTheme);
        applyTheme(css);
        applyChroma(css);
      } catch (e) {
        // Theme resolution failed — fall back to the bundled default and keep
        // the error visible in the status bar so the user knows the flag was
        // unrecognised, without blocking the file from opening.
        themeError = String(e);
        applyTheme(defaultDarkCSS);
        applyChroma(defaultDarkCSS);
      }
    } else {
      // No --theme flag: use the bundled default (no network round-trip).
      applyTheme(defaultDarkCSS);
      applyChroma(defaultDarkCSS);
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

      if (dirty) {
        // The user has unsaved edits. Don't silently discard either version —
        // show the conflict overlay and let them choose.
        showConflict = true;
      } else {
        // Clean state: re-render silently. No user action required.
        loadFile(currentPath);
      }
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
  });

  function handlePlaceholderClick(e) {
    const el = e.target.closest('.remote-image-placeholder');
    if (!el) return;
    loadRemoteImage(el);
  }

  function handlePlaceholderKeydown(e) {
    if (e.key !== 'Enter' && e.key !== ' ') return;
    const el = e.target.closest('.remote-image-placeholder');
    if (!el) return;
    e.preventDefault();
    loadRemoteImage(el);
  }

  function loadRemoteImage(el) {
    const src = el.dataset.src;
    if (!src) return;
    const img = document.createElement('img');
    img.alt = el.textContent.replace('[image] ', '').replace(' (click to load)', '').trim();
    img.src = src;
    img.className = 'remote-image-loaded';
    el.replaceWith(img);
  }

  // Clean up event listeners on unmount
  onDestroy(() => {
    window.removeEventListener('keydown', handleKeydown);
    document.removeEventListener('click', handlePlaceholderClick);
    document.removeEventListener('keydown', handlePlaceholderKeydown);
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
  >
    {@html html}
  </article>

  <!-- Conflict overlay: shown when the file changes on disk while dirty === true.
       In v0.1 dirty is always false so this overlay never appears — but the
       structure is wired so v1.0 edit mode just needs to flip dirty to true. -->
  {#if showConflict}
    <div class="conflict-overlay">
      <div class="conflict-dialog">
        <p>This file was changed on disk. What would you like to do?</p>
        <div class="conflict-actions">
          <button class="conflict-btn" on:click={keepMine}>
            Keep my edits
          </button>
          <button class="conflict-btn primary" on:click={reloadFromDisk}>
            Reload from disk
          </button>
        </div>
      </div>
    </div>
  {/if}

  <footer class="status-bar">
    <span>{wordCount} words</span>
    <span>{readingMins} min read</span>
    {#if needsMermaid}<span class="flag">mermaid</span>{/if}
    {#if needsMath}<span class="flag">math</span>{/if}
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
  /*
    This block contains only rules that need Svelte's component scoping.
    As of milestone 5 there are no such rules — all structural classes live
    in style.css (structure) and the theme file (skin), and the conflict
    overlay classes are already defined in style.css.

    This block is intentionally empty. It exists as a placeholder so the
    pattern is clear for any future component-scoped rules.
  */
</style>