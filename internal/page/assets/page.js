// Runs inside the preview page; mdreader drives it through window.mdr.
(() => {
  const content = document.getElementById('content');

  // Block elements carry data-line (their 1-based source line). Their page
  // positions are cached until the layout may have changed.
  let anchors = null;

  function collectAnchors() {
    // A virtual anchor at the top lets lines before the first block
    // interpolate from the page start.
    const list = [{ line: 0, top: 0 }];
    for (const el of content.querySelectorAll('[data-line]')) {
      const line = Number(el.dataset.line);
      // Skip what's out of source order (footnotes are rendered at the end)
      // and nested blocks starting on the same line as their parent.
      if (line <= list[list.length - 1].line) continue;
      list.push({ line, top: el.getBoundingClientRect().top + window.scrollY });
    }
    return list;
  }

  function invalidate() {
    anchors = null;
  }
  window.addEventListener('resize', invalidate);
  // Images change the layout once they've loaded; load doesn't bubble.
  document.addEventListener('load', invalidate, true);

  window.mdr = {
    update(html) {
      content.innerHTML = html;
      invalidate();
    },

    // Scrolls so that source line `line` appears `percent`% of the window
    // height from the top, interpolating between the surrounding blocks.
    scrollTo(line, percent) {
      anchors ??= collectAnchors();
      let lo = 0;
      let hi = anchors.length;
      while (hi - lo > 1) {
        const mid = (lo + hi) >> 1;
        if (anchors[mid].line <= line) lo = mid;
        else hi = mid;
      }
      const cur = anchors[lo];
      const next = anchors[lo + 1];
      let y = cur.top;
      if (next) y += ((next.top - cur.top) * (line - cur.line)) / (next.line - cur.line);
      window.scrollTo(0, y - (window.innerHeight * percent) / 100);
    },
  };

  // Links never navigate the viewer itself: in-page anchors scroll,
  // everything else is handed to the editor through the mdrOpen binding
  // (resolved against <base>, so relative links arrive as absolute file://
  // URLs).
  document.addEventListener('click', (ev) => {
    const a = ev.target.closest('a[href]');
    if (!a) return;
    ev.preventDefault();
    const raw = a.getAttribute('href');
    if (raw.startsWith('#')) {
      const target = document.getElementById(decodeURIComponent(raw.slice(1)));
      if (target) target.scrollIntoView();
      return;
    }
    if (window.mdrOpen) window.mdrOpen(a.href);
  });

  window.mdrReady?.();
})();
