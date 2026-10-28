// Runs inside the preview page; mdreader drives it through window.mdr.
(() => {
  const content = document.getElementById('content');

  // Block elements carry data-line (their 1-based source line). Their page
  // positions, and the rows of text rendered in them, are cached until the
  // layout may have changed.
  let anchors = null;

  function collectAnchors() {
    // A virtual anchor at the top lets lines before the first block
    // interpolate from the page start.
    const list = [{ line: 0, top: 0, el: null }];
    for (const el of content.querySelectorAll('[data-line]')) {
      const line = Number(el.dataset.line);
      // Skip what's out of source order (footnotes are rendered at the end)
      // and nested blocks starting on the same line as their parent.
      if (line <= list[list.length - 1].line) continue;
      list.push({ line, top: el.getBoundingClientRect().top + window.scrollY, el });
    }
    return list;
  }

  // The rows of text in an anchor's block, as [top, bottom] page positions
  // in order. Boxes of the same row (e.g. a word in <code>) are merged.
  function rows(anchor) {
    if (anchor.rows) return anchor.rows;
    const boxes = [];
    if (anchor.el) {
      const range = document.createRange();
      const walker = document.createTreeWalker(anchor.el, NodeFilter.SHOW_TEXT);
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        if (!node.data.trim()) continue;
        range.selectNodeContents(node);
        for (const r of range.getClientRects()) {
          if (r.height > 0) boxes.push([r.top + window.scrollY, r.bottom + window.scrollY]);
        }
      }
    }
    boxes.sort((a, b) => a[0] - b[0]);
    anchor.rows = [];
    for (const [top, bottom] of boxes) {
      const last = anchor.rows[anchor.rows.length - 1];
      if (last && top < last[1]) last[1] = Math.max(last[1], bottom);
      else anchor.rows.push([top, bottom]);
    }
    return anchor.rows;
  }

  // The index of the last anchor for which key(anchor) <= value.
  function search(key, value) {
    anchors ??= collectAnchors();
    let lo = 0;
    let hi = anchors.length;
    while (hi - lo > 1) {
      const mid = (lo + hi) >> 1;
      if (key(anchors[mid]) <= value) lo = mid;
      else hi = mid;
    }
    return lo;
  }

  // The row of text at page position y, if any.
  function rowAt(y) {
    const i = search((a) => a.top, y);
    // y may lie in a parent block, after a nested one that started later.
    for (const anchor of [anchors[i], anchors[i - 1]]) {
      const row = anchor && rows(anchor).find(([top, bottom]) => top <= y && y < bottom);
      if (row) return row;
    }
    return null;
  }

  function lineHeight() {
    return parseFloat(getComputedStyle(content).lineHeight) || 24;
  }

  // Where source line `line` is rendered, as page positions: interpolated
  // between the blocks around it, widened to the row of text there.
  function locate(line) {
    const i = search((a) => a.line, line);
    const cur = anchors[i];
    const next = anchors[i + 1];
    let y = cur.top;
    if (next) y += ((next.top - cur.top) * (line - cur.line)) / (next.line - cur.line);
    const row = rowAt(y);
    return row ? { top: row[0], bottom: row[1] } : { top: y, bottom: y + lineHeight() };
  }

  // Glides the page to position y: each frame covers part of the way that's
  // left, so a new target simply takes over from wherever the page is.
  const GLIDE_MS = 50; // time constant: 95% of the way after 150 ms
  let glide = null; // { pos, target, time } while gliding

  function glideTo(y, instant) {
    if (instant) {
      glide = null;
      window.scrollTo(0, y);
    } else if (glide) {
      glide.target = y;
    } else {
      glide = { pos: window.scrollY, target: y, time: null };
      requestAnimationFrame(step);
    }
  }

  function step(now) {
    if (!glide) return;
    const dt = glide.time === null ? 16 : now - glide.time;
    glide.time = now;
    glide.pos += (glide.target - glide.pos) * (1 - Math.exp(-dt / GLIDE_MS));
    if (Math.abs(glide.target - glide.pos) < 0.5) glide.pos = glide.target;
    window.scrollTo(0, glide.pos);
    if (glide.pos === glide.target) glide = null;
    else requestAnimationFrame(step);
  }

  // Scrolling by hand stops a glide, and the page stays where it was put
  // until the editor's view changes.
  let following = null; // the last follow() arguments, until the user scrolls
  let followed = false; // whether the page was positioned before
  for (const type of ['wheel', 'touchstart', 'keydown', 'mousedown']) {
    window.addEventListener(
      type,
      () => {
        glide = null;
        following = null;
      },
      { passive: true },
    );
  }

  function invalidate() {
    anchors = null;
    // The layout changed under the editor's view; show it again.
    if (following) follow(...following);
  }
  window.addEventListener('resize', invalidate);
  // Images change the layout once they've loaded; load doesn't bubble.
  document.addEventListener('load', invalidate, true);

  // Shows what the editor shows: source line `top` at the top of the
  // window, and the cursor's line a few rows away from the edges.
  function follow(top, cursor) {
    following = [top, cursor];
    const instant = !followed;
    followed = true;
    const height = window.innerHeight;
    const margin = Math.min(2 * lineHeight(), height / 4);
    const c = locate(cursor);
    let y = locate(top).top;
    y = Math.max(y, c.bottom + margin - height);
    y = Math.min(y, c.top - margin);
    // Don't cut a row of text in half at the top edge: show it whole, or,
    // if more than half of it is out of sight, not at all.
    const row = rowAt(y);
    if (row) y = y - row[0] < row[1] - y ? row[0] : row[1];
    const max = document.documentElement.scrollHeight - height;
    glideTo(Math.max(0, Math.min(y, max)), instant);
  }

  window.mdr = {
    update(html) {
      content.innerHTML = html;
      anchors = null;
    },

    // Makes relative URLs in later updates resolve against baseURI.
    setBase(baseURI) {
      document.querySelector('base').href = baseURI;
    },

    follow,
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
